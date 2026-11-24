package store

import (
	"context"
	"errors"
	"net/url"
	"path"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/research/specter-reimplementation/internal/domain"
)

func (s *Store) Scripts(ctx context.Context) ([]domain.SpecterScript, error) {
	rows, err := s.pool.Query(ctx, `SELECT s.scriptid,s.name,s.description,s.documentation,s.scriptblock,
		s.version,s.ispremium,s.enabled,
		COALESCE(array_agg(l.name ORDER BY l.name) FILTER (WHERE l.name IS NOT NULL),'{}')
		FROM script s
		LEFT JOIN scriptlabel sl ON sl.fkey_scriptid=s.id
		LEFT JOIN label l ON l.id=sl.fkey_labelid
		GROUP BY s.id ORDER BY s.name,s.scriptid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.SpecterScript{}
	for rows.Next() {
		var value domain.SpecterScript
		if err := rows.Scan(&value.Identifier, &value.Name, &value.Description, &value.Documentation,
			&value.ScriptBlock, &value.Version, &value.IsPremium, &value.Enabled, &value.Labels); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) PutScript(ctx context.Context, value domain.SpecterScript) error {
	if value.Identifier == "" || len(value.Identifier) > 128 || value.Name == "" || len(value.Name) > 256 ||
		len(value.Description) > 4096 || len(value.Documentation) > 1<<20 || len(value.ScriptBlock) > 1<<20 {
		return errors.New("invalid script")
	}
	if value.Version < 1 {
		value.Version = 1
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var scriptID int64
	err = tx.QueryRow(ctx, `INSERT INTO script(scriptid,name,description,documentation,scriptblock,version,ispremium,enabled)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT(scriptid) DO UPDATE SET name=excluded.name,description=excluded.description,
		documentation=excluded.documentation,scriptblock=excluded.scriptblock,version=excluded.version,
		ispremium=excluded.ispremium,enabled=excluded.enabled RETURNING id`, value.Identifier, value.Name,
		value.Description, value.Documentation, value.ScriptBlock, value.Version, value.IsPremium, value.Enabled).Scan(&scriptID)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM scriptlabel WHERE fkey_scriptid=$1", scriptID); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, raw := range value.Labels {
		label := strings.TrimSpace(raw)
		if label == "" || len(label) > 64 || seen[label] {
			continue
		}
		seen[label] = true
		var labelID int64
		if err := tx.QueryRow(ctx, `INSERT INTO label(name) VALUES($1)
			ON CONFLICT(name) DO UPDATE SET name=excluded.name RETURNING id`, label).Scan(&labelID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO scriptlabel(fkey_scriptid,fkey_labelid) VALUES($1,$2)", scriptID, labelID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) DeleteScript(ctx context.Context, identifier string) error {
	command, err := s.pool.Exec(ctx, "DELETE FROM script WHERE scriptid=$1", identifier)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) Wordlists(ctx context.Context, tags []string) ([]domain.WordlistResponse, error) {
	query := `SELECT id,name,description,tags,COALESCE(content,''),createdat,updatedat FROM wordlist`
	args := []any{}
	if len(tags) > 0 {
		query += " WHERE tags @> $1"
		args = append(args, tags)
	}
	query += " ORDER BY name,id"
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.WordlistResponse{}
	for rows.Next() {
		value, err := scanWordlist(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) Wordlist(ctx context.Context, id int64) (domain.WordlistResponse, error) {
	value, err := scanWordlist(s.pool.QueryRow(ctx, `SELECT id,name,description,tags,COALESCE(content,''),createdat,updatedat
		FROM wordlist WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.WordlistResponse{}, ErrNotFound
	}
	return value, err
}

func (s *Store) CreateWordlist(ctx context.Context, request domain.AddWordlistRequest) (domain.WordlistResponse, error) {
	if err := validateWordlist(request); err != nil {
		return domain.WordlistResponse{}, err
	}
	var id int64
	err := s.pool.QueryRow(ctx, `INSERT INTO wordlist(name,description,tags,content)
		VALUES($1,$2,$3,$4) RETURNING id`, request.Name, request.Description, request.Tags, request.Content).Scan(&id)
	if err != nil {
		return domain.WordlistResponse{}, err
	}
	return s.Wordlist(ctx, id)
}

func (s *Store) UpdateWordlist(ctx context.Context, id int64, request domain.AddWordlistRequest) (domain.WordlistResponse, error) {
	if err := validateWordlist(request); err != nil {
		return domain.WordlistResponse{}, err
	}
	command, err := s.pool.Exec(ctx, `UPDATE wordlist SET name=$2,description=$3,tags=$4,content=$5,updatedat=now()
		WHERE id=$1`, id, request.Name, request.Description, request.Tags, request.Content)
	if err != nil {
		return domain.WordlistResponse{}, err
	}
	if command.RowsAffected() == 0 {
		return domain.WordlistResponse{}, ErrNotFound
	}
	return s.Wordlist(ctx, id)
}

func (s *Store) DeleteWordlist(ctx context.Context, id int64) error {
	command, err := s.pool.Exec(ctx, "DELETE FROM wordlist WHERE id=$1", id)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

type wordlistScanner interface{ Scan(...any) error }

func scanWordlist(row wordlistScanner) (domain.WordlistResponse, error) {
	var value domain.WordlistResponse
	err := row.Scan(&value.ID, &value.Name, &value.Description, &value.Tags, &value.Content, &value.CreatedAt, &value.UpdatedAt)
	value.EntryCount = nonEmptyLineCount(value.Content)
	return value, err
}

func validateWordlist(value domain.AddWordlistRequest) error {
	if strings.TrimSpace(value.Name) == "" || len(value.Name) > 256 || len(value.Description) > 4096 || len(value.Content) > 8<<20 || len(value.Tags) > 64 {
		return errors.New("invalid wordlist")
	}
	for _, tag := range value.Tags {
		if strings.TrimSpace(tag) == "" || len(tag) > 64 {
			return errors.New("invalid wordlist tag")
		}
	}
	return nil
}

func nonEmptyLineCount(content string) int {
	count := 0
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func (s *Store) Listeners(ctx context.Context) ([]domain.Listener, error) {
	rows, err := s.pool.Query(ctx, `SELECT prefix,enabled,certificatename,COALESCE(webdavpath,'') FROM listener ORDER BY prefix`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.Listener{}
	for rows.Next() {
		var value domain.Listener
		if err := rows.Scan(&value.Prefix, &value.Enabled, &value.CertificateName, &value.WebDavPath); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) CreateListener(ctx context.Context, request domain.AddHTTPListenerRequest) error {
	if err := validateListener(request.Prefix, request.CertificateName, request.WebDavPath); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO listener(prefix,enabled,certificatename,webdavpath) VALUES($1,true,$2,NULLIF($3,''))`,
		request.Prefix, request.CertificateName, request.WebDavPath)
	return err
}

func (s *Store) UpdateListener(ctx context.Context, request domain.UpdateListenerRequest) error {
	command, err := s.pool.Exec(ctx, "UPDATE listener SET enabled=$2 WHERE prefix=$1", request.Prefix, request.Enabled)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) DeleteListener(ctx context.Context, prefix string) error {
	command, err := s.pool.Exec(ctx, "DELETE FROM listener WHERE prefix=$1", prefix)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func validateListener(prefix, certificateName, webDAVPath string) error {
	if prefix == "" || len(prefix) > 2048 || len(certificateName) > 256 || len(webDAVPath) > 2048 {
		return errors.New("invalid listener")
	}
	parsed, err := url.ParseRequestURI(prefix)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || !strings.HasSuffix(parsed.Path, "/") || hasParentSegment(parsed.Path) {
		return errors.New("listener prefix must be an absolute http(s) URL without traversal, credentials, query, or fragment")
	}
	if parsed.Scheme == "https" && strings.TrimSpace(certificateName) == "" {
		return errors.New("https listener requires a certificate name")
	}
	if webDAVPath != "" && (!strings.HasPrefix(webDAVPath, "/") || path.Clean(webDAVPath) != webDAVPath || hasParentSegment(webDAVPath)) {
		return errors.New("invalid WebDAV path")
	}
	return nil
}

func hasParentSegment(value string) bool {
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func (s *Store) Certificates(ctx context.Context) ([]domain.CertificateInfo, error) {
	rows, err := s.pool.Query(ctx, `SELECT name,country,state,locality,organization,organizational_unit,
		common_name,valid_from,valid_until FROM app_certificates ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.CertificateInfo{}
	for rows.Next() {
		var value domain.CertificateInfo
		if err := rows.Scan(&value.Name, &value.Country, &value.State, &value.Locality, &value.Organization,
			&value.OrganizationalUnit, &value.CommonName, &value.Start, &value.End); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) PutCertificate(ctx context.Context, value domain.CertificateInfo) error {
	if value.Name == "" || len(value.Name) > 256 || value.CommonName == "" || len(value.CommonName) > 512 ||
		value.Start.IsZero() || !value.End.After(value.Start) {
		return errors.New("invalid certificate metadata")
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO app_certificates(name,country,state,locality,organization,
		organizational_unit,common_name,valid_from,valid_until) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT(name) DO UPDATE SET country=excluded.country,state=excluded.state,locality=excluded.locality,
		organization=excluded.organization,organizational_unit=excluded.organizational_unit,
		common_name=excluded.common_name,valid_from=excluded.valid_from,valid_until=excluded.valid_until`,
		value.Name, value.Country, value.State, value.Locality, value.Organization, value.OrganizationalUnit,
		value.CommonName, value.Start, value.End)
	return err
}

func (s *Store) DeleteCertificate(ctx context.Context, name string) error {
	command, err := s.pool.Exec(ctx, "DELETE FROM app_certificates WHERE name=$1", name)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
