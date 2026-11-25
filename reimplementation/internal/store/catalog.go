package store

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

func (s *Store) Modules(ctx context.Context) ([]domain.SpecterModuleInfo, error) {
	rows, err := s.pool.Query(ctx, `SELECT name,version,runtime_version,cmdlets FROM app_modules ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.SpecterModuleInfo{}
	for rows.Next() {
		var value domain.SpecterModuleInfo
		if err := rows.Scan(&value.Name, &value.Version, &value.RuntimeVersion, &value.Cmdlets); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) PutModule(ctx context.Context, name string, content []byte, digest string) error {
	if strings.TrimSpace(name) == "" || len(name) > 256 || len(content) == 0 || len(content) > 4<<20 || len(digest) != 64 {
		return errors.New("invalid module blob")
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO app_modules(name,version,runtime_version,cmdlets,content_sha256,content)
		VALUES($1,'opaque','not-inspected','{}',$2,$3)
		ON CONFLICT(name) DO UPDATE SET content_sha256=excluded.content_sha256,content=excluded.content,
		version=excluded.version,runtime_version=excluded.runtime_version,cmdlets=excluded.cmdlets,updated_at=now()`, name, digest, content)
	return err
}

func (s *Store) ModuleContent(ctx context.Context, name string) ([]byte, string, error) {
	var content []byte
	var digest string
	err := s.pool.QueryRow(ctx, `SELECT content,content_sha256 FROM app_modules WHERE name=$1`, name).Scan(&content, &digest)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	return content, digest, err
}

func (s *Store) DeleteModule(ctx context.Context, name string) error {
	command, err := s.pool.Exec(ctx, "DELETE FROM app_modules WHERE name=$1", name)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) Notebooks(ctx context.Context) ([]domain.PowerShellNotebook, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,notebookid,name,description,timecreated,timeupdated,cells
		FROM powershellnotebooks ORDER BY name,notebookid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.PowerShellNotebook{}
	for rows.Next() {
		value, err := scanNotebook(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) Notebook(ctx context.Context, notebookID string) (domain.PowerShellNotebook, error) {
	value, err := scanNotebook(s.pool.QueryRow(ctx, `SELECT id,notebookid,name,description,timecreated,timeupdated,cells
		FROM powershellnotebooks WHERE notebookid=$1`, notebookID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PowerShellNotebook{}, ErrNotFound
	}
	return value, err
}

func (s *Store) CreateNotebook(ctx context.Context, request domain.NotebookRequest) (domain.PowerShellNotebook, error) {
	if err := validateNotebook(request); err != nil {
		return domain.PowerShellNotebook{}, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		notebookID := randomStoreID("notebook")
		cells, _ := json.Marshal(request.Cells)
		_, err := s.pool.Exec(ctx, `INSERT INTO powershellnotebooks(notebookid,name,description,timecreated,timeupdated,cells)
			VALUES($1,$2,$3,now(),now(),$4)`, notebookID, request.Name, request.Description, cells)
		if err == nil {
			return s.Notebook(ctx, notebookID)
		}
		if !isUniqueViolation(err) {
			return domain.PowerShellNotebook{}, err
		}
	}
	return domain.PowerShellNotebook{}, errors.New("could not allocate notebook id")
}

func (s *Store) UpdateNotebook(ctx context.Context, notebookID string, request domain.NotebookRequest) (domain.PowerShellNotebook, error) {
	if err := validateNotebook(request); err != nil {
		return domain.PowerShellNotebook{}, err
	}
	cells, _ := json.Marshal(request.Cells)
	command, err := s.pool.Exec(ctx, `UPDATE powershellnotebooks SET name=$2,description=$3,cells=$4,timeupdated=now()
		WHERE notebookid=$1`, notebookID, request.Name, request.Description, cells)
	if err != nil {
		return domain.PowerShellNotebook{}, err
	}
	if command.RowsAffected() == 0 {
		return domain.PowerShellNotebook{}, ErrNotFound
	}
	return s.Notebook(ctx, notebookID)
}

func (s *Store) UpsertNotebook(ctx context.Context, notebookID string, request domain.NotebookRequest) (domain.PowerShellNotebook, error) {
	if strings.TrimSpace(notebookID) == "" || len(notebookID) > 128 || strings.ContainsAny(notebookID, "/\\") {
		return domain.PowerShellNotebook{}, errors.New("invalid notebook id")
	}
	if err := validateNotebook(request); err != nil {
		return domain.PowerShellNotebook{}, err
	}
	cells, _ := json.Marshal(request.Cells)
	_, err := s.pool.Exec(ctx, `INSERT INTO powershellnotebooks(notebookid,name,description,timecreated,timeupdated,cells)
		VALUES($1,$2,$3,now(),now(),$4)
		ON CONFLICT(notebookid) DO UPDATE SET name=excluded.name,description=excluded.description,
		cells=excluded.cells,timeupdated=now()`, notebookID, request.Name, request.Description, cells)
	if err != nil {
		return domain.PowerShellNotebook{}, err
	}
	return s.Notebook(ctx, notebookID)
}

func (s *Store) DeleteNotebook(ctx context.Context, notebookID string) error {
	command, err := s.pool.Exec(ctx, "DELETE FROM powershellnotebooks WHERE notebookid=$1", notebookID)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

type notebookScanner interface{ Scan(...any) error }

func scanNotebook(row notebookScanner) (domain.PowerShellNotebook, error) {
	var value domain.PowerShellNotebook
	var cells []byte
	err := row.Scan(&value.ID, &value.NotebookID, &value.Name, &value.Description, &value.TimeCreated, &value.TimeUpdated, &cells)
	if err != nil {
		return domain.PowerShellNotebook{}, err
	}
	if err := json.Unmarshal(cells, &value.Cells); err != nil {
		return domain.PowerShellNotebook{}, err
	}
	if value.Cells == nil {
		value.Cells = []json.RawMessage{}
	}
	return value, nil
}

func validateNotebook(value domain.NotebookRequest) error {
	if strings.TrimSpace(value.Name) == "" || len(value.Name) > 256 || len(value.Description) > 4096 || len(value.Cells) > 256 {
		return errors.New("invalid notebook")
	}
	total := 0
	for _, cell := range value.Cells {
		total += len(cell)
		if !json.Valid(cell) || len(cell) > 256<<10 {
			return errors.New("invalid notebook cell")
		}
	}
	if total > 2<<20 {
		return errors.New("notebook cells exceed 2 MiB")
	}
	return nil
}

func randomStoreID(prefix string) string {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, len(prefix))
	}
	return fmt.Sprintf("%s-%x", prefix, value[:])
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
