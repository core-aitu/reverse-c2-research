package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/research/specter-reimplementation/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *Store) Migrate(ctx context.Context, directory string) error {
	if _, err := s.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		var exists bool
		if err := s.pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", name,
		).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		migration, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			return err
		}
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(migration)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", name); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) EnsureAdmin(ctx context.Context, username, password string) error {
	if password == "" {
		return nil
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM account WHERE username=$1)", username).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var accountID int64
	if err := tx.QueryRow(ctx,
		"INSERT INTO account(username, hash, enabled) VALUES($1,$2,true) RETURNING id",
		username, string(hash),
	).Scan(&accountID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO app_account_roles(account_id, role) VALUES($1,'admin')", accountID,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) Authenticate(ctx context.Context, username, password string) (domain.Account, error) {
	var account domain.Account
	var hash string
	err := s.pool.QueryRow(ctx,
		"SELECT id, username, enabled, hash FROM account WHERE username=$1", username,
	).Scan(&account.ID, &account.Username, &account.Enabled, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Account{}, ErrNotFound
	}
	if err != nil || !account.Enabled || bcrypt.CompareHashAndPassword([]byte(strings.TrimSpace(hash)), []byte(password)) != nil {
		return domain.Account{}, ErrNotFound
	}
	account.Roles, err = s.roles(ctx, account.ID)
	return account, err
}

func (s *Store) AccountByID(ctx context.Context, id int64) (domain.Account, error) {
	var account domain.Account
	err := s.pool.QueryRow(ctx, "SELECT id, username, enabled FROM account WHERE id=$1", id).
		Scan(&account.ID, &account.Username, &account.Enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Account{}, ErrNotFound
	}
	if err != nil {
		return domain.Account{}, err
	}
	account.Roles, err = s.roles(ctx, id)
	return account, err
}

func (s *Store) roles(ctx context.Context, accountID int64) ([]string, error) {
	rows, err := s.pool.Query(ctx, "SELECT role FROM app_account_roles WHERE account_id=$1 ORDER BY role", accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roles := []string{}
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (s *Store) Accounts(ctx context.Context) ([]domain.Account, error) {
	rows, err := s.pool.Query(ctx, "SELECT id, username, enabled FROM account ORDER BY username")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := []domain.Account{}
	for rows.Next() {
		var account domain.Account
		if err := rows.Scan(&account.ID, &account.Username, &account.Enabled); err != nil {
			return nil, err
		}
		account.Roles, err = s.roles(ctx, account.ID)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func tokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Store) SaveRefreshToken(ctx context.Context, accountID int64, token, ip string, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO refreshtokens
		(account_id, token, created, expires, created_by_ip, revoked)
		VALUES($1,$2,now(),$3,$4,false)`, accountID, tokenDigest(token), expires, ip)
	return err
}

func (s *Store) RotateRefreshToken(ctx context.Context, token, replacement, ip string, expires time.Time) (domain.Account, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Account{}, err
	}
	defer tx.Rollback(ctx)
	var accountID int64
	err = tx.QueryRow(ctx, `UPDATE refreshtokens SET revoked=true
		WHERE token=$1 AND revoked=false AND expires>now() RETURNING account_id`, tokenDigest(token)).Scan(&accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Account{}, ErrNotFound
	}
	if err != nil {
		return domain.Account{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO refreshtokens
		(account_id, token, created, expires, created_by_ip, revoked)
		VALUES($1,$2,now(),$3,$4,false)`, accountID, tokenDigest(replacement), expires, ip); err != nil {
		return domain.Account{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Account{}, err
	}
	return s.AccountByID(ctx, accountID)
}

func (s *Store) RevokeRefreshToken(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, "UPDATE refreshtokens SET revoked=true WHERE token=$1", tokenDigest(token))
	return err
}

func (s *Store) Documentation(ctx context.Context) ([]domain.DocumentationEntry, error) {
	rows, err := s.pool.Query(ctx, "SELECT path, title, content FROM documentationchunks WHERE chunkindex=0 ORDER BY path")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []domain.DocumentationEntry{}
	for rows.Next() {
		var path, title, content string
		if err := rows.Scan(&path, &title, &content); err != nil {
			return nil, err
		}
		entries = append(entries, domain.DocumentationEntry{Name: path, Contents: content, Children: []domain.DocumentationEntry{}})
	}
	return entries, rows.Err()
}

func (s *Store) SearchDocumentation(ctx context.Context, query string, limit int) ([]domain.DocumentationSearchResult, error) {
	if limit < 1 || limit > 50 {
		limit = 10
	}
	rows, err := s.pool.Query(ctx, `SELECT path, title, content,
		GREATEST(similarity(title,$1), similarity(content,$1)) AS score
		FROM documentationchunks
		WHERE title ILIKE '%' || $1 || '%' OR content ILIKE '%' || $1 || '%'
		ORDER BY score DESC, path LIMIT $2`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []domain.DocumentationSearchResult{}
	for rows.Next() {
		var result domain.DocumentationSearchResult
		if err := rows.Scan(&result.Path, &result.Title, &result.Content, &result.Score); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

func (s *Store) SeedDocumentation(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO documentationchunks(path,title,content,chunkindex)
		VALUES
		('Overview.md','Overview','Clean-room SpecterInsight research environment.',0),
		('Guide/Sample.md','Sample','The first vertical slice includes auth, docs, events and settings.',0)
		ON CONFLICT (path,chunkindex) DO NOTHING`)
	return err
}

func (s *Store) AddEvent(ctx context.Context, event domain.Event) (domain.Event, error) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	err := s.pool.QueryRow(ctx, `INSERT INTO event
		(identifier,timestamp,type,source,outcome,system,message,details)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		event.Identifier, event.Timestamp, event.Type, event.Source, event.Outcome,
		event.System, event.Message, event.Details,
	).Scan(&event.ID)
	return event, err
}

func (s *Store) EventsAfter(ctx context.Context, after int64, limit int) ([]domain.Event, error) {
	if limit < 1 || limit > 1000 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `SELECT id,identifier,timestamp,type,source,outcome,system,message,details
		FROM event WHERE id>$1 ORDER BY id LIMIT $2`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []domain.Event{}
	for rows.Next() {
		var event domain.Event
		if err := rows.Scan(&event.ID, &event.Identifier, &event.Timestamp, &event.Type,
			&event.Source, &event.Outcome, &event.System, &event.Message, &event.Details); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) LatestEvents(ctx context.Context, from *time.Time, limit int) ([]domain.Event, error) {
	if limit < 1 || limit > 1000 {
		limit = 200
	}
	query := `SELECT id,identifier,timestamp,type,source,outcome,system,message,details FROM event`
	args := []any{limit}
	if from != nil {
		query += " WHERE timestamp >= $2"
		args = append(args, *from)
	}
	query += " ORDER BY id DESC LIMIT $1"
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []domain.Event{}
	for rows.Next() {
		var event domain.Event
		if err := rows.Scan(&event.ID, &event.Identifier, &event.Timestamp, &event.Type,
			&event.Source, &event.Outcome, &event.System, &event.Message, &event.Details); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) Settings(ctx context.Context) ([]domain.Setting, error) {
	rows, err := s.pool.Query(ctx, "SELECT key,value FROM app_settings ORDER BY key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	settings := []domain.Setting{}
	for rows.Next() {
		var setting domain.Setting
		if err := rows.Scan(&setting.Key, &setting.Value); err != nil {
			return nil, err
		}
		settings = append(settings, setting)
	}
	return settings, rows.Err()
}

func (s *Store) PutSettings(ctx context.Context, settings []domain.Setting) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, setting := range settings {
		if setting.Key == "" || len(setting.Key) > 128 || len(setting.Value) > 4096 {
			return errors.New("invalid setting")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app_settings(key,value,updated_at) VALUES($1,$2,now())
			ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`,
			setting.Key, setting.Value); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
