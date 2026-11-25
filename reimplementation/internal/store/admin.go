package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/research/specter-reimplementation/internal/domain"
)

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
	_, err := s.pool.Exec(ctx, `UPDATE app_update_state SET status='Complete',status_description='fixture check complete; no package downloaded',
		last_update_result='NoUpdateAvailable',progress=100,updated_at=now() WHERE singleton=true`)
	return err
}
