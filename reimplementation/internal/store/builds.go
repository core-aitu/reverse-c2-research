package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/research/specter-reimplementation/internal/domain"
)

const beaconReleaseColumns = `r.id,r.version,r.os,r.architecture,r.signed,r.development,r.created_at,
	o.id,o.kind,o.object_key,o.original_name,o.sha256,o.media_type,o.size_bytes,o.created_at`

func scanBeaconRelease(row pgx.Row) (domain.BeaconRelease, error) {
	var value domain.BeaconRelease
	err := row.Scan(&value.ID, &value.Version, &value.OS, &value.Architecture, &value.Signed, &value.Development, &value.CreatedAt,
		&value.Object.ID, &value.Object.Kind, &value.Object.Key, &value.Object.Name, &value.Object.SHA256, &value.Object.MediaType, &value.Object.Size, &value.Object.CreatedAt)
	return value, err
}

func (s *Store) RegisterBeaconRelease(ctx context.Context, id, version, operatingSystem, architecture, objectID string, development bool) (domain.BeaconRelease, error) {
	if id == "" || version == "" || objectID == "" || !validReleaseTarget(operatingSystem, architecture) {
		return domain.BeaconRelease{}, errors.New("invalid beacon release")
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO app_beacon_releases(id,version,os,architecture,object_id,signed,development)
		VALUES($1,$2,$3,$4,$5,false,$6)
		ON CONFLICT(version,os,architecture) DO UPDATE SET object_id=EXCLUDED.object_id,sbom_object_id=NULL,checksums_object_id=NULL,signature_object_id=NULL,signed=false,development=EXCLUDED.development,created_at=now()`,
		id, version, operatingSystem, architecture, objectID, development)
	if err != nil {
		return domain.BeaconRelease{}, err
	}
	return s.BeaconReleaseByTarget(ctx, version, operatingSystem, architecture)
}

func (s *Store) BeaconReleaseByTarget(ctx context.Context, version, operatingSystem, architecture string) (domain.BeaconRelease, error) {
	value, err := scanBeaconRelease(s.pool.QueryRow(ctx, `SELECT `+beaconReleaseColumns+`
		FROM app_beacon_releases r JOIN app_objects o ON o.id=r.object_id
		WHERE r.version=$1 AND r.os=$2 AND r.architecture=$3`, version, operatingSystem, architecture))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.BeaconRelease{}, ErrNotFound
	}
	if err == nil {
		err = s.decorateBeaconRelease(ctx, &value)
	}
	return value, err
}

func (s *Store) BeaconRelease(ctx context.Context, id string) (domain.BeaconRelease, error) {
	value, err := scanBeaconRelease(s.pool.QueryRow(ctx, `SELECT `+beaconReleaseColumns+`
		FROM app_beacon_releases r JOIN app_objects o ON o.id=r.object_id WHERE r.id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.BeaconRelease{}, ErrNotFound
	}
	if err == nil {
		err = s.decorateBeaconRelease(ctx, &value)
	}
	return value, err
}

func (s *Store) BeaconReleases(ctx context.Context) ([]domain.BeaconRelease, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+beaconReleaseColumns+`
		FROM app_beacon_releases r JOIN app_objects o ON o.id=r.object_id
		ORDER BY r.created_at DESC,r.os,r.architecture`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.BeaconRelease{}
	for rows.Next() {
		value, err := scanBeaconRelease(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for index := range values {
		if err = s.decorateBeaconRelease(ctx, &values[index]); err != nil {
			return nil, err
		}
	}
	return values, nil
}

func (s *Store) decorateBeaconRelease(ctx context.Context, value *domain.BeaconRelease) error {
	var sbomID, checksumsID, signatureID *string
	if err := s.pool.QueryRow(ctx, `SELECT sbom_object_id,checksums_object_id,signature_object_id FROM app_beacon_releases WHERE id=$1`, value.ID).Scan(&sbomID, &checksumsID, &signatureID); err != nil {
		return err
	}
	load := func(id *string) (*domain.ObjectInfo, error) {
		if id == nil {
			return nil, nil
		}
		object, err := s.ObjectInfo(ctx, *id)
		if err != nil {
			return nil, err
		}
		return &domain.ObjectInfo{ID: object.ID, Kind: object.Kind, Key: object.Key, Name: object.OriginalName, SHA256: object.SHA256, MediaType: object.MediaType, Size: object.Size, CreatedAt: object.CreatedAt}, nil
	}
	var err error
	if value.SBOM, err = load(sbomID); err != nil {
		return err
	}
	if value.Checksums, err = load(checksumsID); err != nil {
		return err
	}
	if value.Signature, err = load(signatureID); err != nil {
		return err
	}
	value.Signed = value.Signature != nil
	value.SignatureStatus = "missing"
	if value.Signed {
		value.SignatureStatus = "present"
	}
	return nil
}

func (s *Store) SetBeaconReleaseAsset(ctx context.Context, releaseID, asset, objectID string) (domain.BeaconRelease, error) {
	column := ""
	switch asset {
	case "sbom":
		column = "sbom_object_id"
	case "checksums":
		column = "checksums_object_id"
	case "signature":
		column = "signature_object_id"
	default:
		return domain.BeaconRelease{}, errors.New("invalid beacon release asset")
	}
	query := `UPDATE app_beacon_releases SET ` + column + `=$2,signed=(CASE WHEN $3='signature' THEN true ELSE signed END) WHERE id=$1`
	command, err := s.pool.Exec(ctx, query, releaseID, objectID, asset)
	if err != nil {
		return domain.BeaconRelease{}, err
	}
	if command.RowsAffected() == 0 {
		return domain.BeaconRelease{}, ErrNotFound
	}
	return s.BeaconRelease(ctx, releaseID)
}

func (s *Store) CreateBeaconBuild(ctx context.Context, id, releaseID, createdBy string) (domain.BeaconBuild, error) {
	if id == "" || releaseID == "" || createdBy == "" {
		return domain.BeaconBuild{}, errors.New("invalid beacon build")
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO app_beacon_builds(id,release_id,created_by) VALUES($1,$2,$3)`, id, releaseID, createdBy)
	if err != nil {
		return domain.BeaconBuild{}, err
	}
	return s.BeaconBuild(ctx, id)
}

func (s *Store) BeaconBuild(ctx context.Context, id string) (domain.BeaconBuild, error) {
	var value domain.BeaconBuild
	err := s.pool.QueryRow(ctx, `SELECT b.id,b.release_id,r.version,r.os,r.architecture,o.sha256,b.created_by,b.created_at,b.last_bundle_at
		FROM app_beacon_builds b JOIN app_beacon_releases r ON r.id=b.release_id JOIN app_objects o ON o.id=r.object_id WHERE b.id=$1`, id).
		Scan(&value.ID, &value.ReleaseID, &value.Version, &value.OS, &value.Architecture, &value.SHA256, &value.CreatedBy, &value.CreatedAt, &value.LastBundleAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.BeaconBuild{}, ErrNotFound
	}
	return value, err
}

func (s *Store) MarkBeaconBundle(ctx context.Context, id string, at time.Time) error {
	command, err := s.pool.Exec(ctx, `UPDATE app_beacon_builds SET last_bundle_at=$2 WHERE id=$1`, id, at)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func validReleaseTarget(operatingSystem, architecture string) bool {
	return (operatingSystem == "linux" && (architecture == "amd64" || architecture == "arm64")) ||
		(operatingSystem == "windows" && architecture == "amd64") ||
		(operatingSystem == "darwin" && architecture == "arm64")
}
