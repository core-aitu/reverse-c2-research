package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/research/specter-reimplementation/internal/domain"
	"github.com/research/specter-reimplementation/internal/gateway"
)

func (s *Store) CreateAgentEnrollment(ctx context.Context, id, token, createdBy string, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO app_agent_enrollment_tokens(id,token_hash,expires_at,created_by) VALUES($1,$2,$3,$4)`, id, tokenDigest(token), expires, createdBy)
	return err
}

func (s *Store) AgentEnrollmentTokens(ctx context.Context) ([]domain.EnrollmentTokenInfo, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,expires_at,used_at,created_by,created_at FROM app_agent_enrollment_tokens ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.EnrollmentTokenInfo{}
	for rows.Next() {
		var value domain.EnrollmentTokenInfo
		if err := rows.Scan(&value.ID, &value.ExpiresAt, &value.UsedAt, &value.CreatedBy, &value.CreatedAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) AgentIdentities(ctx context.Context) ([]domain.AgentIdentity, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,name,generation,enabled,revoked_at,metadata,last_sequence,last_cursor,first_seen,last_checkin FROM app_agent_identities ORDER BY last_checkin DESC,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.AgentIdentity{}
	for rows.Next() {
		var value domain.AgentIdentity
		if err := rows.Scan(&value.ID, &value.Name, &value.Generation, &value.Enabled, &value.RevokedAt, &value.Metadata, &value.LastSequence, &value.LastCursor, &value.FirstSeen, &value.LastCheckin); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) AgentIdentity(ctx context.Context, id string) (domain.AgentIdentity, error) {
	var value domain.AgentIdentity
	err := s.pool.QueryRow(ctx, `SELECT id,name,generation,enabled,revoked_at,metadata,last_sequence,last_cursor,first_seen,last_checkin
		FROM app_agent_identities WHERE id=$1`, id).Scan(&value.ID, &value.Name, &value.Generation, &value.Enabled, &value.RevokedAt, &value.Metadata, &value.LastSequence, &value.LastCursor, &value.FirstSeen, &value.LastCheckin)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AgentIdentity{}, ErrNotFound
	}
	return value, err
}

func (s *Store) EnrollAgent(ctx context.Context, enrollmentToken, id, name, identity string, metadata json.RawMessage) (domain.AgentIdentity, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.AgentIdentity{}, err
	}
	defer tx.Rollback(ctx)
	var tokenID string
	err = tx.QueryRow(ctx, `SELECT id FROM app_agent_enrollment_tokens WHERE token_hash=$1 AND used_at IS NULL AND expires_at>now() FOR UPDATE`, tokenDigest(enrollmentToken)).Scan(&tokenID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AgentIdentity{}, ErrNotFound
	}
	if err != nil {
		return domain.AgentIdentity{}, err
	}
	var value domain.AgentIdentity
	err = tx.QueryRow(ctx, `INSERT INTO app_agent_identities(id,name,identity_hash,metadata) VALUES($1,$2,$3,$4)
		RETURNING id,name,generation,enabled,revoked_at,metadata,last_sequence,last_cursor,first_seen,last_checkin`, id, name, tokenDigest(identity), metadata).Scan(&value.ID, &value.Name, &value.Generation, &value.Enabled, &value.RevokedAt, &value.Metadata, &value.LastSequence, &value.LastCursor, &value.FirstSeen, &value.LastCheckin)
	if err != nil {
		return domain.AgentIdentity{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE app_agent_enrollment_tokens SET used_at=now() WHERE id=$1`, tokenID); err != nil {
		return domain.AgentIdentity{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.AgentIdentity{}, err
	}
	return value, nil
}

func (s *Store) AuthenticateAgent(ctx context.Context, identity string) (domain.AgentIdentity, error) {
	var value domain.AgentIdentity
	err := s.pool.QueryRow(ctx, `SELECT id,name,generation,enabled,revoked_at,metadata,last_sequence,last_cursor,first_seen,last_checkin
		FROM app_agent_identities WHERE (identity_hash=$1 OR (previous_identity_hash=$1 AND previous_identity_valid_until>now())) AND enabled=true AND revoked_at IS NULL`, tokenDigest(identity)).Scan(&value.ID, &value.Name, &value.Generation, &value.Enabled, &value.RevokedAt, &value.Metadata, &value.LastSequence, &value.LastCursor, &value.FirstSeen, &value.LastCheckin)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AgentIdentity{}, ErrNotFound
	}
	return value, err
}

func (s *Store) AdvanceAgentSequence(ctx context.Context, id string, sequence uint64, checkin bool) error {
	query := `UPDATE app_agent_identities SET last_sequence=$2 WHERE id=$1 AND enabled=true AND revoked_at IS NULL AND last_sequence<$2`
	if checkin {
		query = `UPDATE app_agent_identities SET last_sequence=$2,last_checkin=now() WHERE id=$1 AND enabled=true AND revoked_at IS NULL AND last_sequence<$2`
	}
	command, err := s.pool.Exec(ctx, query, id, sequence)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) AdvanceAgentCheckin(ctx context.Context, id string, sequence, cursor uint64) error {
	command, err := s.pool.Exec(ctx, `UPDATE app_agent_identities SET last_sequence=$2,last_cursor=GREATEST(last_cursor,$3),last_checkin=now()
		WHERE id=$1 AND enabled=true AND revoked_at IS NULL AND last_sequence<$2`, id, sequence, cursor)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) RotateAgentIdentity(ctx context.Context, id, identity string) error {
	command, err := s.pool.Exec(ctx, `UPDATE app_agent_identities SET previous_identity_hash=identity_hash,previous_identity_valid_until=now()+interval '5 minutes',identity_hash=$2,generation=generation+1 WHERE id=$1 AND revoked_at IS NULL`, id, tokenDigest(identity))
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) SetAgentEnabled(ctx context.Context, id string, enabled bool) error {
	command, err := s.pool.Exec(ctx, `UPDATE app_agent_identities SET enabled=$2 WHERE id=$1 AND revoked_at IS NULL`, id, enabled)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) RevokeAgent(ctx context.Context, id string) error {
	command, err := s.pool.Exec(ctx, `UPDATE app_agent_identities SET enabled=false,revoked_at=now() WHERE id=$1 AND revoked_at IS NULL`, id)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) CreateGatewayTask(ctx context.Context, value domain.GatewayTask) (domain.GatewayTask, error) {
	if value.Version == 0 {
		value.Version = gateway.TaskEnvelopeVersion
	}
	if !gateway.ValidTaskKindForVersion(value.Version, value.Kind) || value.IdempotencyKey == "" || len(value.IdempotencyKey) > 128 || len(value.Payload) == 0 || len(value.Payload) > gateway.MaxTaskSize || !json.Valid(value.Payload) {
		return domain.GatewayTask{}, errors.New("invalid bounded task envelope")
	}
	if value.TimeoutMS == 0 {
		value.TimeoutMS = int64(time.Minute / time.Millisecond)
	}
	timeout := time.Duration(value.TimeoutMS) * time.Millisecond
	if timeout < gateway.MinTaskTimeout || timeout > gateway.MaxTaskTimeout || value.Deadline.Before(time.Now().UTC()) || value.Deadline.After(time.Now().UTC().Add(24*time.Hour)) {
		return domain.GatewayTask{}, errors.New("task deadline or timeout exceeds policy")
	}
	return scanGatewayTask(s.pool.QueryRow(ctx, `INSERT INTO app_agent_gateway_tasks(id,agent_id,envelope_version,idempotency_key,kind,payload,status,deadline,timeout_ms)
		VALUES($1,$2,$3,$4,$5,$6,'queued',$7,$8)
		RETURNING `+gatewayTaskColumns, value.ID, value.AgentID, value.Version, value.IdempotencyKey, value.Kind, value.Payload, value.Deadline, value.TimeoutMS))
}

func (s *Store) GatewayTasks(ctx context.Context, agentID string) ([]domain.GatewayTask, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+gatewayTaskColumns+` FROM app_agent_gateway_tasks WHERE agent_id=$1 ORDER BY created_at DESC LIMIT 500`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.GatewayTask{}
	for rows.Next() {
		value, err := scanGatewayTask(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) LeaseGatewayTasks(ctx context.Context, agentID string, after uint64, limit uint32, lease time.Duration) ([]domain.GatewayTask, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if limit == 0 || limit > gateway.MaxTasksPerCheckin {
		return nil, errors.New("invalid lease limit")
	}
	if _, err = tx.Exec(ctx, `UPDATE app_agent_gateway_tasks SET status='failed',error_code='deadline_exceeded',error_message='task expired before dispatch',error_retryable=false,completed_at=now(),lease_until=NULL
		WHERE agent_id=$1 AND status IN ('queued','leased') AND deadline<=now()`, agentID); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE app_agent_gateway_tasks SET status='cancelled',error_code='cancelled',error_message='task cancellation requested',error_retryable=false,completed_at=now(),lease_until=NULL
		WHERE agent_id=$1 AND status IN ('leased','running') AND cancel_requested_at IS NOT NULL AND lease_until<now()`, agentID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT `+gatewayTaskColumns+`
		FROM app_agent_gateway_tasks WHERE agent_id=$1 AND cursor>$2 AND cancel_requested_at IS NULL AND deadline>now()
		AND (status='queued' OR (status IN ('leased','running') AND lease_until<now()))
		ORDER BY cursor FOR UPDATE SKIP LOCKED LIMIT $3`, agentID, after, limit)
	if err != nil {
		return nil, err
	}
	values := []domain.GatewayTask{}
	for rows.Next() {
		v, scanErr := scanGatewayTask(rows)
		if scanErr != nil {
			rows.Close()
			return nil, scanErr
		}
		values = append(values, v)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for i := range values {
		if err = tx.QueryRow(ctx, `UPDATE app_agent_gateway_tasks SET status='leased',lease_until=now()+$2::interval,dispatch_count=dispatch_count+1 WHERE id=$1 RETURNING lease_until,dispatch_count`, values[i].ID, lease.String()).Scan(&values[i].LeaseUntil, &values[i].DispatchCount); err != nil {
			return nil, err
		}
		values[i].Status = "leased"
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return values, nil
}

func (s *Store) AckGatewayTask(ctx context.Context, agentID, taskID string, cursor uint64) error {
	command, err := s.pool.Exec(ctx, `UPDATE app_agent_gateway_tasks SET status='running',acknowledged_at=COALESCE(acknowledged_at,now()),started_at=COALESCE(started_at,now())
		WHERE id=$1 AND agent_id=$2 AND cursor=$3 AND status='leased' AND deadline>now()`, taskID, agentID, cursor)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) CompleteGatewayTask(ctx context.Context, agentID string, taskID, submissionID string, sequence uint64, success bool, result json.RawMessage, code, message string, retryable bool) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	digestInput := fmt.Sprintf("%t\n%s\n%s\n%s\n%t", success, result, code, message, retryable)
	digestBytes := sha256.Sum256([]byte(digestInput))
	submissionDigest := hex.EncodeToString(digestBytes[:])
	var status, existing, existingDigest string
	var cursor uint64
	var cancelRequested *time.Time
	err = tx.QueryRow(ctx, `SELECT status,COALESCE(submission_id,''),COALESCE(submission_digest,''),cursor,cancel_requested_at FROM app_agent_gateway_tasks WHERE id=$1 AND agent_id=$2 FOR UPDATE`, taskID, agentID).Scan(&status, &existing, &existingDigest, &cursor, &cancelRequested)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	if status == "completed" || status == "failed" || status == "cancelled" {
		if existing == submissionID && existingDigest == submissionDigest {
			return true, nil
		}
		return false, ErrNotFound
	}
	if status != "running" && status != "leased" {
		return false, ErrNotFound
	}
	command, err := tx.Exec(ctx, `UPDATE app_agent_identities SET last_sequence=$2,last_cursor=GREATEST(last_cursor,$3) WHERE id=$1 AND enabled=true AND revoked_at IS NULL AND last_sequence<$2`, agentID, sequence, cursor)
	if err != nil {
		return false, err
	}
	if command.RowsAffected() == 0 {
		return false, ErrNotFound
	}
	newStatus := "failed"
	if success {
		newStatus = "completed"
	}
	if cancelRequested != nil {
		newStatus, success, code, message, retryable = "cancelled", false, "cancelled", "task cancellation requested", false
	}
	_, err = tx.Exec(ctx, `UPDATE app_agent_gateway_tasks SET status=$3,submission_id=$4,submission_digest=$5,result=$6,error_code=$7,error_message=$8,error_retryable=$9,completed_at=now(),lease_until=NULL WHERE id=$1 AND agent_id=$2`, taskID, agentID, newStatus, submissionID, submissionDigest, result, code, message, retryable)
	if err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return false, nil
}

func (s *Store) CancelGatewayTask(ctx context.Context, agentID, taskID string) (string, error) {
	var status string
	err := s.pool.QueryRow(ctx, `UPDATE app_agent_gateway_tasks SET
		cancel_requested_at=COALESCE(cancel_requested_at,now()),
		status=CASE WHEN status='queued' THEN 'cancelled' ELSE status END,
		completed_at=CASE WHEN status='queued' THEN now() ELSE completed_at END,
		lease_until=CASE WHEN status='queued' THEN NULL ELSE lease_until END,
		error_code=CASE WHEN status='queued' THEN 'cancelled' ELSE error_code END,
		error_message=CASE WHEN status='queued' THEN 'task cancelled before execution' ELSE error_message END
		WHERE id=$1 AND agent_id=$2 AND status IN ('queued','leased','running') RETURNING status`, taskID, agentID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return status, err
}

func (s *Store) GatewayTaskCancelRequested(ctx context.Context, agentID, taskID string) (bool, error) {
	var requested bool
	err := s.pool.QueryRow(ctx, `SELECT cancel_requested_at IS NOT NULL OR status='cancelled' FROM app_agent_gateway_tasks WHERE id=$1 AND agent_id=$2`, taskID, agentID).Scan(&requested)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	return requested, err
}

func (s *Store) UseAgentNonce(ctx context.Context, agentID, nonce string, sequence uint64, cutoff time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM app_agent_request_nonces WHERE observed_at<$1`, cutoff); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO app_agent_request_nonces(agent_id,nonce,sequence) VALUES($1,$2,$3)`, agentID, nonce, sequence); err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return ErrNotFound
		}
		return err
	}
	return tx.Commit(ctx)
}

const gatewayTaskColumns = `id,agent_id,cursor,envelope_version,idempotency_key,kind,payload,status,lease_until,deadline,timeout_ms,cancel_requested_at,started_at,dispatch_count,result,error_code,error_message,error_retryable,created_at,completed_at`

type rowScanner interface {
	Scan(...any) error
}

func scanGatewayTask(row rowScanner) (domain.GatewayTask, error) {
	var value domain.GatewayTask
	err := row.Scan(&value.ID, &value.AgentID, &value.Cursor, &value.Version, &value.IdempotencyKey, &value.Kind, &value.Payload, &value.Status, &value.LeaseUntil, &value.Deadline, &value.TimeoutMS, &value.CancelRequestedAt, &value.StartedAt, &value.DispatchCount, &value.Result, &value.ErrorCode, &value.ErrorMessage, &value.ErrorRetryable, &value.CreatedAt, &value.CompletedAt)
	return value, err
}
