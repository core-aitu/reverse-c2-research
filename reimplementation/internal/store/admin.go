package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/research/specter-reimplementation/internal/domain"
	"github.com/research/specter-reimplementation/internal/objectstore"
)

const (
	MaxArtifactContent = 16 << 20
	MaxUpdatePackage   = 32 << 20
)

type ObjectRecord struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	Key          string    `json:"-"`
	SHA256       string    `json:"sha256"`
	Size         int64     `json:"size"`
	MediaType    string    `json:"mediaType"`
	OriginalName string    `json:"name"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (s *Store) PutObject(ctx context.Context, kind, name, mediaType string, reader io.Reader, size int64) (ObjectRecord, error) {
	if s.objects == nil {
		return ObjectRecord{}, errors.New("object storage is not configured")
	}
	info, err := s.objects.Put(ctx, kind, name, mediaType, reader, size)
	if err != nil {
		return ObjectRecord{}, err
	}
	return s.recordObject(ctx, info)
}

func (s *Store) recordObject(ctx context.Context, info objectstore.Info) (ObjectRecord, error) {
	idDigest := sha256.Sum256([]byte(info.Kind + "\x00" + info.SHA256))
	id := "obj_" + hex.EncodeToString(idDigest[:16])
	value := ObjectRecord{ID: id, Kind: info.Kind, Key: info.Key, SHA256: info.SHA256, Size: info.Size, MediaType: info.MediaType, OriginalName: info.Name}
	err := s.pool.QueryRow(ctx, `INSERT INTO app_objects(id,kind,object_key,sha256,size_bytes,media_type,original_name)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT(kind,sha256) DO UPDATE SET original_name=EXCLUDED.original_name
		RETURNING id,kind,object_key,sha256,size_bytes,media_type,original_name,created_at`,
		id, info.Kind, info.Key, info.SHA256, info.Size, info.MediaType, info.Name).
		Scan(&value.ID, &value.Kind, &value.Key, &value.SHA256, &value.Size, &value.MediaType, &value.OriginalName, &value.CreatedAt)
	return value, err
}

func (s *Store) BeginObjectMultipart(ctx context.Context, kind, name, mediaType string) (objectstore.MultipartUpload, error) {
	if s.objects == nil {
		return objectstore.MultipartUpload{}, errors.New("object storage is not configured")
	}
	return s.objects.BeginMultipart(ctx, kind, name, mediaType)
}

func (s *Store) PutObjectPart(ctx context.Context, upload objectstore.MultipartUpload, number int, reader io.Reader, size int64) (objectstore.MultipartPart, error) {
	if s.objects == nil {
		return objectstore.MultipartPart{}, errors.New("object storage is not configured")
	}
	return s.objects.PutPart(ctx, upload, number, reader, size)
}

func (s *Store) CompleteObjectMultipart(ctx context.Context, upload objectstore.MultipartUpload, parts []objectstore.MultipartPart, digest string, size int64) (ObjectRecord, error) {
	if s.objects == nil {
		return ObjectRecord{}, errors.New("object storage is not configured")
	}
	info, err := s.objects.CompleteMultipart(ctx, upload, parts, digest, size)
	if err != nil {
		return ObjectRecord{}, err
	}
	return s.recordObject(ctx, info)
}

func (s *Store) AbortObjectMultipart(ctx context.Context, upload objectstore.MultipartUpload) error {
	if s.objects == nil {
		return errors.New("object storage is not configured")
	}
	return s.objects.AbortMultipart(ctx, upload)
}

func (s *Store) putObjectBytes(ctx context.Context, kind, name, mediaType string, content []byte) (ObjectRecord, error) {
	return s.PutObject(ctx, kind, name, mediaType, bytes.NewReader(content), int64(len(content)))
}

func (s *Store) ObjectInfo(ctx context.Context, id string) (ObjectRecord, error) {
	var value ObjectRecord
	err := s.pool.QueryRow(ctx, `SELECT id,kind,object_key,sha256,size_bytes,media_type,original_name,created_at FROM app_objects WHERE id=$1`, id).
		Scan(&value.ID, &value.Kind, &value.Key, &value.SHA256, &value.Size, &value.MediaType, &value.OriginalName, &value.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ObjectRecord{}, ErrNotFound
	}
	return value, err
}

func (s *Store) OpenObject(ctx context.Context, id string, start, end int64) (io.ReadCloser, ObjectRecord, error) {
	value, err := s.ObjectInfo(ctx, id)
	if err != nil {
		return nil, ObjectRecord{}, err
	}
	if s.objects == nil {
		return nil, ObjectRecord{}, errors.New("object storage is not configured")
	}
	reader, err := s.objects.Open(ctx, value.Key, start, end)
	return reader, value, err
}

func (s *Store) readObject(ctx context.Context, id string, maximum int64) ([]byte, error) {
	reader, value, err := s.OpenObject(ctx, id, 0, -1)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	if value.Size > maximum {
		return nil, errors.New("stored object exceeds policy")
	}
	content, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil || int64(len(content)) != value.Size {
		return nil, io.ErrUnexpectedEOF
	}
	return content, nil
}

// MigrateLegacyObjects is restart-safe: rows are switched to S3 only after the
// content-addressed object and its metadata row have been written.
func (s *Store) MigrateLegacyObjects(ctx context.Context, batch int) (int, error) {
	if s.objects == nil {
		return 0, errors.New("object storage is not configured")
	}
	if batch < 1 || batch > 1000 {
		batch = 100
	}
	type legacy struct {
		table, key, kind, name, mediaType, digest string
		content                                   []byte
	}
	values := []legacy{}
	rows, err := s.pool.Query(ctx, `SELECT c.artifact_id::text,'artifact',a.name,c.media_type,c.sha256,c.content
		FROM app_artifact_contents c JOIN artifacts a ON a.id=c.artifact_id
		WHERE c.object_id IS NULL AND c.content IS NOT NULL ORDER BY c.artifact_id LIMIT $1`, batch)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var value legacy
		value.table = "artifact"
		if err := rows.Scan(&value.key, &value.kind, &value.name, &value.mediaType, &value.digest, &value.content); err != nil {
			rows.Close()
			return 0, err
		}
		values = append(values, value)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	remaining := batch - len(values)
	if remaining > 0 {
		rows, err = s.pool.Query(ctx, `SELECT version_key,'update-package',version_key,media_type,sha256,content
			FROM app_update_packages WHERE object_id IS NULL AND content IS NOT NULL ORDER BY version_key LIMIT $1`, remaining)
		if err != nil {
			return 0, err
		}
		for rows.Next() {
			var value legacy
			value.table = "update"
			if err := rows.Scan(&value.key, &value.kind, &value.name, &value.mediaType, &value.digest, &value.content); err != nil {
				rows.Close()
				return 0, err
			}
			values = append(values, value)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return 0, err
		}
	}
	migrated := 0
	for _, value := range values {
		object, err := s.putObjectBytes(ctx, value.kind, value.name, value.mediaType, value.content)
		if err != nil {
			return migrated, err
		}
		if value.digest != "" && subtle.ConstantTimeCompare([]byte(strings.ToLower(value.digest)), []byte(object.SHA256)) != 1 {
			return migrated, errors.New("legacy object digest mismatch")
		}
		reader, _, err := s.OpenObject(ctx, object.ID, 0, -1)
		if err != nil {
			return migrated, err
		}
		hash := sha256.New()
		read, verifyErr := io.Copy(hash, reader)
		closeErr := reader.Close()
		if verifyErr != nil || closeErr != nil || read != object.Size || hex.EncodeToString(hash.Sum(nil)) != object.SHA256 {
			return migrated, errors.New("uploaded legacy object verification failed")
		}
		var command string
		if value.table == "artifact" {
			command = `UPDATE app_artifact_contents SET object_id=$2,content=NULL WHERE artifact_id=$1::integer AND object_id IS NULL`
		} else {
			command = `UPDATE app_update_packages SET object_id=$2,content=NULL WHERE version_key=$1 AND object_id IS NULL`
		}
		result, err := s.pool.Exec(ctx, command, value.key, object.ID)
		if err != nil {
			return migrated, err
		}
		if result.RowsAffected() == 1 {
			migrated++
		}
	}
	return migrated, nil
}

func (s *Store) Artifacts(ctx context.Context, host string) ([]domain.ArtifactInfo, error) {
	query := `SELECT COALESCE(source,''),name,filesize,createdtimestamp FROM artifacts`
	args := []any{}
	if host != "" {
		query += " WHERE COALESCE(source,'')=$1"
		args = append(args, host)
	}
	query += " ORDER BY createdtimestamp DESC,name"
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.ArtifactInfo{}
	for rows.Next() {
		var value domain.ArtifactInfo
		if err := rows.Scan(&value.Host, &value.Name, &value.Length, &value.CreationDate); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) DeleteArtifact(ctx context.Context, host, name string) error {
	command, err := s.pool.Exec(ctx, "DELETE FROM artifacts WHERE COALESCE(source,'')=$1 AND name=$2", host, name)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) PutArtifactContent(ctx context.Context, host, name, mediaType string, content []byte) (domain.ArtifactContentInfo, error) {
	if s.objects == nil {
		return domain.ArtifactContentInfo{}, errors.New("object storage is required for new artifact content")
	}
	host, name = strings.TrimSpace(host), strings.TrimSpace(name)
	if !safeContentKey(host, 128) || !safeContentKey(name, 255) {
		return domain.ArtifactContentInfo{}, errors.New("invalid artifact host or name")
	}
	if len(content) == 0 || len(content) > MaxArtifactContent {
		return domain.ArtifactContentInfo{}, fmt.Errorf("artifact content must be between 1 byte and %d bytes", MaxArtifactContent)
	}
	if mediaType == "" || len(mediaType) > 128 {
		mediaType = "application/octet-stream"
	}
	digest := sha256.Sum256(content)
	digestText := hex.EncodeToString(digest[:])
	contentLength := int64(len(content))
	object, err := s.putObjectBytes(ctx, "artifact", name, mediaType, content)
	if err != nil {
		return domain.ArtifactContentInfo{}, err
	}
	objectID := object.ID
	content = nil
	now := time.Now().UTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.ArtifactContentInfo{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "DELETE FROM artifacts WHERE COALESCE(source,'')=$1 AND name=$2", host, name); err != nil {
		return domain.ArtifactContentInfo{}, err
	}
	var artifactID int64
	err = tx.QueryRow(ctx, `INSERT INTO artifacts(sha256,name,extension,username,filesize,sourcetype,source,sourcepath,createdtimestamp,ingestedtimestamp)
		VALUES($1,$2,'','', $3,'clean-room-ingest',$4,'',$5,$5) RETURNING id`, digestText, name, contentLength, host, now).Scan(&artifactID)
	if err != nil {
		return domain.ArtifactContentInfo{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app_artifact_contents(artifact_id,sha256,media_type,content,object_id,created_at)
		VALUES($1,$2,$3,$4,$5,$6)`, artifactID, digestText, mediaType, content, objectID, now); err != nil {
		return domain.ArtifactContentInfo{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ArtifactContentInfo{}, err
	}
	return domain.ArtifactContentInfo{ArtifactInfo: domain.ArtifactInfo{Host: host, Name: name, Length: contentLength, CreationDate: now}, SHA256: digestText, MediaType: mediaType}, nil
}

func (s *Store) ArtifactContent(ctx context.Context, host, name string) ([]byte, domain.ArtifactContentInfo, error) {
	var content []byte
	var objectID *string
	var value domain.ArtifactContentInfo
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(a.source,''),a.name,a.filesize,a.createdtimestamp,c.sha256,c.media_type,c.content,c.object_id
		FROM artifacts a JOIN app_artifact_contents c ON c.artifact_id=a.id
		WHERE COALESCE(a.source,'')=$1 AND a.name=$2 ORDER BY a.id DESC LIMIT 1`, host, name).
		Scan(&value.Host, &value.Name, &value.Length, &value.CreationDate, &value.SHA256, &value.MediaType, &content, &objectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ArtifactContentInfo{}, ErrNotFound
	}
	if err == nil && objectID != nil {
		content, err = s.readObject(ctx, *objectID, MaxArtifactContent)
	}
	return content, value, err
}

func (s *Store) License(ctx context.Context) (domain.LicenseValidationInfoEx, error) {
	var email, digest, hint string
	err := s.pool.QueryRow(ctx, "SELECT email,key_digest,key_hint FROM app_license_state WHERE singleton=true").Scan(&email, &digest, &hint)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.LicenseValidationInfoEx{}, ErrNotFound
	}
	value := domain.LicenseValidationInfoEx{Key: hint, Success: false, License: "unvalidated-clean-room-metadata",
		ItemID: "research", ItemName: "SpecterInsight clean-room reimplementation", Checksum: digest,
		Expires: time.Unix(0, 0).UTC(), CustomerEmail: email, LicenseLimit: "0", SiteCount: "0", ActivationsLeft: "0"}
	return value, err
}

func (s *Store) PutLicense(ctx context.Context, value domain.LicenseInfo) (domain.LicenseValidationInfoEx, error) {
	if len(value.Email) > 320 || len(value.Key) < 8 || len(value.Key) > 4096 || !strings.Contains(value.Email, "@") {
		return domain.LicenseValidationInfoEx{}, errors.New("invalid license metadata")
	}
	digest := sha256.Sum256([]byte(value.Key))
	hint := value.Key
	if len(hint) > 8 {
		hint = hint[:4] + "…" + hint[len(hint)-4:]
	}
	_, err := s.pool.Exec(ctx, `UPDATE app_license_state SET email=$1,key_digest=$2,key_hint=$3,updated_at=now() WHERE singleton=true`,
		value.Email, hex.EncodeToString(digest[:]), hint)
	if err != nil {
		return domain.LicenseValidationInfoEx{}, err
	}
	return s.License(ctx)
}

func (s *Store) Version(ctx context.Context) (domain.SerializableVersion, error) {
	var value domain.SerializableVersion
	err := s.pool.QueryRow(ctx, "SELECT major,minor,build,revision FROM app_update_state WHERE singleton=true").
		Scan(&value.Major, &value.Minor, &value.Build, &value.Revision)
	return value, err
}

func (s *Store) UpdateStatus(ctx context.Context) (domain.UpdateJobInfo, error) {
	var value domain.UpdateJobInfo
	err := s.pool.QueryRow(ctx, `SELECT major,minor,build,revision,status,status_description,last_update_result,progress
		FROM app_update_state WHERE singleton=true`).Scan(&value.Version.Major, &value.Version.Minor, &value.Version.Build,
		&value.Version.Revision, &value.Status, &value.StatusDescription, &value.LastUpdateResult, &value.Progress)
	return value, err
}

func (s *Store) CheckUpdates(ctx context.Context) error {
	value, err := s.ActiveUpdatePackage(ctx)
	if errors.Is(err, ErrNotFound) {
		_, err = s.pool.Exec(ctx, `UPDATE app_update_state SET status='Complete',status_description='package catalog checked; no package available',
			last_update_result='NoUpdateAvailable',progress=100,updated_at=now() WHERE singleton=true`)
		return err
	}
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE app_update_state SET major=$1,minor=$2,build=$3,revision=$4,status='Complete',
		status_description='verified package available: '||$5,last_update_result='UpdateAvailable',progress=100,updated_at=now()
		WHERE singleton=true`, value.Version.Major, value.Version.Minor, value.Version.Build, value.Version.Revision, value.SHA256)
	return err
}

func (s *Store) PutUpdatePackage(ctx context.Context, versionKey, mediaType, expectedDigest string, content []byte) (domain.UpdatePackageInfo, error) {
	if s.objects == nil {
		return domain.UpdatePackageInfo{}, errors.New("object storage is required for new update packages")
	}
	version, err := parseVersion(versionKey)
	if err != nil {
		return domain.UpdatePackageInfo{}, err
	}
	if len(content) == 0 || len(content) > MaxUpdatePackage {
		return domain.UpdatePackageInfo{}, fmt.Errorf("update package must be between 1 byte and %d bytes", MaxUpdatePackage)
	}
	digest := sha256.Sum256(content)
	digestText := hex.EncodeToString(digest[:])
	contentLength := int64(len(content))
	expectedDigest = strings.ToLower(strings.TrimSpace(expectedDigest))
	if len(expectedDigest) != 64 || subtle.ConstantTimeCompare([]byte(expectedDigest), []byte(digestText)) != 1 {
		return domain.UpdatePackageInfo{}, errors.New("X-Content-SHA256 does not match update package")
	}
	if mediaType == "" || len(mediaType) > 128 {
		mediaType = "application/octet-stream"
	}
	object, objectErr := s.putObjectBytes(ctx, "update-package", versionKey, mediaType, content)
	if objectErr != nil {
		return domain.UpdatePackageInfo{}, objectErr
	}
	objectID := object.ID
	content = nil
	now := time.Now().UTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.UpdatePackageInfo{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "UPDATE app_update_packages SET active=false WHERE active=true"); err != nil {
		return domain.UpdatePackageInfo{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO app_update_packages(version_key,major,minor,build,revision,sha256,media_type,content,object_id,active,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,true,$10)
		ON CONFLICT(version_key) DO UPDATE SET sha256=EXCLUDED.sha256,media_type=EXCLUDED.media_type,content=EXCLUDED.content,object_id=EXCLUDED.object_id,active=true,created_at=EXCLUDED.created_at`,
		versionKey, version.Major, version.Minor, version.Build, version.Revision, digestText, mediaType, content, objectID, now)
	if err != nil {
		return domain.UpdatePackageInfo{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.UpdatePackageInfo{}, err
	}
	return domain.UpdatePackageInfo{Version: version, SHA256: digestText, MediaType: mediaType, Length: contentLength, Active: true, CreatedAt: now}, nil
}

func (s *Store) UpdatePackages(ctx context.Context) ([]domain.UpdatePackageInfo, error) {
	rows, err := s.pool.Query(ctx, `SELECT p.major,p.minor,p.build,p.revision,p.sha256,p.media_type,COALESCE(o.size_bytes,octet_length(p.content)),p.active,p.created_at
		FROM app_update_packages p LEFT JOIN app_objects o ON o.id=p.object_id ORDER BY p.major DESC,p.minor DESC,p.build DESC,p.revision DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.UpdatePackageInfo{}
	for rows.Next() {
		var value domain.UpdatePackageInfo
		if err := rows.Scan(&value.Version.Major, &value.Version.Minor, &value.Version.Build, &value.Version.Revision,
			&value.SHA256, &value.MediaType, &value.Length, &value.Active, &value.CreatedAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) ActiveUpdatePackage(ctx context.Context) (domain.UpdatePackageInfo, error) {
	var value domain.UpdatePackageInfo
	err := s.pool.QueryRow(ctx, `SELECT p.major,p.minor,p.build,p.revision,p.sha256,p.media_type,COALESCE(o.size_bytes,octet_length(p.content)),p.active,p.created_at
		FROM app_update_packages p LEFT JOIN app_objects o ON o.id=p.object_id WHERE p.active=true`).Scan(&value.Version.Major, &value.Version.Minor, &value.Version.Build,
		&value.Version.Revision, &value.SHA256, &value.MediaType, &value.Length, &value.Active, &value.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.UpdatePackageInfo{}, ErrNotFound
	}
	return value, err
}

func (s *Store) ActiveUpdatePackageContent(ctx context.Context) ([]byte, domain.UpdatePackageInfo, error) {
	var content []byte
	var objectID *string
	var value domain.UpdatePackageInfo
	err := s.pool.QueryRow(ctx, `SELECT p.major,p.minor,p.build,p.revision,p.sha256,p.media_type,COALESCE(o.size_bytes,octet_length(p.content)),p.active,p.created_at,p.content,p.object_id
		FROM app_update_packages p LEFT JOIN app_objects o ON o.id=p.object_id WHERE p.active=true`).Scan(&value.Version.Major, &value.Version.Minor, &value.Version.Build,
		&value.Version.Revision, &value.SHA256, &value.MediaType, &value.Length, &value.Active, &value.CreatedAt, &content, &objectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.UpdatePackageInfo{}, ErrNotFound
	}
	if err == nil && objectID != nil {
		content, err = s.readObject(ctx, *objectID, MaxUpdatePackage)
	}
	return content, value, err
}

func (s *Store) DeleteUpdatePackage(ctx context.Context, versionKey string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var active bool
	if err := tx.QueryRow(ctx, "DELETE FROM app_update_packages WHERE version_key=$1 RETURNING active", versionKey).Scan(&active); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if active {
		if _, err := tx.Exec(ctx, `UPDATE app_update_state SET status='Complete',status_description='active package removed',
			last_update_result='NoUpdateAvailable',progress=100,updated_at=now() WHERE singleton=true`); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func parseVersion(value string) (domain.SerializableVersion, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 4 {
		return domain.SerializableVersion{}, errors.New("version must contain four numeric components")
	}
	numbers := make([]int, 4)
	for index, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 || number > 65535 || strconv.Itoa(number) != part {
			return domain.SerializableVersion{}, errors.New("invalid version component")
		}
		numbers[index] = number
	}
	return domain.SerializableVersion{Major: numbers[0], Minor: numbers[1], Build: numbers[2], Revision: numbers[3]}, nil
}

func safeContentKey(value string, maximum int) bool {
	if value == "" || len(value) > maximum || value == "." || value == ".." || strings.ContainsAny(value, "/\\\x00\r\n") {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}
