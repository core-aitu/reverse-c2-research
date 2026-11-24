package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/research/specter-reimplementation/internal/domain"
)

func (s *Store) Resources(ctx context.Context, kind string) ([]domain.Resource, error) {
	rows, err := s.pool.Query(ctx, `SELECT kind,resource_key,name,description,status,spec,created_at,updated_at
		FROM app_resources WHERE kind=$1 ORDER BY updated_at DESC, resource_key`, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	resources := []domain.Resource{}
	for rows.Next() {
		var value domain.Resource
		if err := rows.Scan(&value.Kind, &value.Key, &value.Name, &value.Description, &value.Status,
			&value.Spec, &value.CreatedAt, &value.UpdatedAt); err != nil {
			return nil, err
		}
		resources = append(resources, value)
	}
	return resources, rows.Err()
}

func (s *Store) PutResource(ctx context.Context, value domain.Resource) (domain.Resource, error) {
	if len(value.Kind) < 1 || len(value.Kind) > 64 || len(value.Key) < 1 || len(value.Key) > 128 ||
		len(value.Name) < 1 || len(value.Name) > 256 || len(value.Description) > 4096 {
		return domain.Resource{}, errors.New("invalid resource")
	}
	if value.Status == "" {
		value.Status = "ready"
	}
	if len(value.Spec) == 0 {
		value.Spec = json.RawMessage(`{}`)
	}
	if !json.Valid(value.Spec) {
		return domain.Resource{}, errors.New("invalid resource spec")
	}
	err := s.pool.QueryRow(ctx, `INSERT INTO app_resources(kind,resource_key,name,description,status,spec)
		VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT(kind,resource_key) DO UPDATE SET name=excluded.name,description=excluded.description,
		status=excluded.status,spec=excluded.spec,updated_at=now()
		RETURNING kind,resource_key,name,description,status,spec,created_at,updated_at`,
		value.Kind, value.Key, value.Name, value.Description, value.Status, value.Spec,
	).Scan(&value.Kind, &value.Key, &value.Name, &value.Description, &value.Status,
		&value.Spec, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

func (s *Store) DeleteResource(ctx context.Context, kind, key string) error {
	command, err := s.pool.Exec(ctx, "DELETE FROM app_resources WHERE kind=$1 AND resource_key=$2", kind, key)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) Adapters(ctx context.Context) ([]domain.Adapter, error) {
	rows, err := s.pool.Query(ctx, `SELECT name,category,mode,enabled,endpoint,status,last_error,checked_at,config,updated_at
		FROM app_adapters ORDER BY category,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.Adapter{}
	for rows.Next() {
		var value domain.Adapter
		if err := rows.Scan(&value.Name, &value.Category, &value.Mode, &value.Enabled, &value.Endpoint,
			&value.Status, &value.LastError, &value.CheckedAt, &value.Config, &value.UpdatedAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) Adapter(ctx context.Context, name string) (domain.Adapter, error) {
	var value domain.Adapter
	err := s.pool.QueryRow(ctx, `SELECT name,category,mode,enabled,endpoint,status,last_error,checked_at,config,updated_at
		FROM app_adapters WHERE name=$1`, name).Scan(&value.Name, &value.Category, &value.Mode, &value.Enabled,
		&value.Endpoint, &value.Status, &value.LastError, &value.CheckedAt, &value.Config, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Adapter{}, ErrNotFound
	}
	return value, err
}

func (s *Store) PutAdapter(ctx context.Context, value domain.Adapter) (domain.Adapter, error) {
	if value.Name == "" || value.Category == "" || (value.Mode != "fixture" && value.Mode != "real") || len(value.Endpoint) > 2048 {
		return domain.Adapter{}, errors.New("invalid adapter")
	}
	if len(value.Config) == 0 {
		value.Config = json.RawMessage(`{}`)
	}
	if !json.Valid(value.Config) {
		return domain.Adapter{}, errors.New("invalid adapter config")
	}
	err := s.pool.QueryRow(ctx, `INSERT INTO app_adapters(name,category,mode,enabled,endpoint,status,last_error,config)
		VALUES($1,$2,$3,$4,$5,'unknown','',$6)
		ON CONFLICT(name) DO UPDATE SET category=excluded.category,mode=excluded.mode,enabled=excluded.enabled,
		endpoint=excluded.endpoint,config=excluded.config,status='unknown',last_error='',updated_at=now()
		RETURNING name,category,mode,enabled,endpoint,status,last_error,checked_at,config,updated_at`,
		value.Name, value.Category, value.Mode, value.Enabled, value.Endpoint, value.Config,
	).Scan(&value.Name, &value.Category, &value.Mode, &value.Enabled, &value.Endpoint, &value.Status,
		&value.LastError, &value.CheckedAt, &value.Config, &value.UpdatedAt)
	return value, err
}

func (s *Store) SetAdapterStatus(ctx context.Context, name, status, detail string) (domain.Adapter, error) {
	_, err := s.pool.Exec(ctx, `UPDATE app_adapters SET status=$2,last_error=$3,checked_at=now(),updated_at=now() WHERE name=$1`, name, status, detail)
	if err != nil {
		return domain.Adapter{}, err
	}
	return s.Adapter(ctx, name)
}

func (s *Store) CreateJob(ctx context.Context, value domain.Job) (domain.Job, error) {
	if len(value.Input) == 0 {
		value.Input = json.RawMessage(`{}`)
	}
	if value.MaxAttempts < 1 {
		value.MaxAttempts = 1
	}
	err := s.pool.QueryRow(ctx, `INSERT INTO app_jobs(id,kind,status,input,max_attempts,requested_by)
		VALUES($1,$2,'queued',$3,$4,$5)
		RETURNING id,kind,status,input,output,error,attempt,max_attempts,cancel_requested,requested_by,
		created_at,started_at,completed_at`, value.ID, value.Kind, value.Input, value.MaxAttempts, value.RequestedBy,
	).Scan(&value.ID, &value.Kind, &value.Status, &value.Input, &value.Output, &value.Error, &value.Attempt,
		&value.MaxAttempts, &value.CancelRequested, &value.RequestedBy, &value.CreatedAt, &value.StartedAt, &value.CompletedAt)
	return value, err
}

func (s *Store) Jobs(ctx context.Context, limit int) ([]domain.Job, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `SELECT id,kind,status,input,output,error,attempt,max_attempts,cancel_requested,
		requested_by,created_at,started_at,completed_at FROM app_jobs ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.Job{}
	for rows.Next() {
		var value domain.Job
		if err := rows.Scan(&value.ID, &value.Kind, &value.Status, &value.Input, &value.Output, &value.Error,
			&value.Attempt, &value.MaxAttempts, &value.CancelRequested, &value.RequestedBy, &value.CreatedAt,
			&value.StartedAt, &value.CompletedAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) StartJob(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE app_jobs SET status='running',attempt=attempt+1,started_at=now()
		WHERE id=$1 AND status='queued' AND cancel_requested=false`, id)
	return err
}

func (s *Store) FinishJob(ctx context.Context, id string, output json.RawMessage, runErr error) error {
	status, message := "succeeded", ""
	if runErr != nil {
		status, message = "failed", runErr.Error()
	}
	_, err := s.pool.Exec(ctx, `UPDATE app_jobs SET status=$2,output=$3,error=$4,completed_at=now()
		WHERE id=$1 AND status='running'`, id, status, output, message)
	return err
}

func (s *Store) CancelJob(ctx context.Context, id string) error {
	command, err := s.pool.Exec(ctx, `UPDATE app_jobs SET cancel_requested=true,
		status=CASE WHEN status='queued' THEN 'cancelled' ELSE status END,
		completed_at=CASE WHEN status='queued' THEN now() ELSE completed_at END
		WHERE id=$1 AND status IN ('queued','running')`, id)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) Pipelines(ctx context.Context) ([]domain.PipelineDefinition, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,name,description,definition,enabled,created_at,updated_at
		FROM app_pipeline_definitions ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.PipelineDefinition{}
	for rows.Next() {
		var value domain.PipelineDefinition
		if err := rows.Scan(&value.ID, &value.Name, &value.Description, &value.Definition, &value.Enabled,
			&value.CreatedAt, &value.UpdatedAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) Pipeline(ctx context.Context, id string) (domain.PipelineDefinition, error) {
	var value domain.PipelineDefinition
	err := s.pool.QueryRow(ctx, `SELECT id,name,description,definition,enabled,created_at,updated_at
		FROM app_pipeline_definitions WHERE id=$1`, id).Scan(&value.ID, &value.Name, &value.Description,
		&value.Definition, &value.Enabled, &value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PipelineDefinition{}, ErrNotFound
	}
	return value, err
}

func (s *Store) PutPipeline(ctx context.Context, value domain.PipelineDefinition) (domain.PipelineDefinition, error) {
	if value.ID == "" || value.Name == "" || !json.Valid(value.Definition) {
		return domain.PipelineDefinition{}, errors.New("invalid pipeline")
	}
	err := s.pool.QueryRow(ctx, `INSERT INTO app_pipeline_definitions(id,name,description,definition,enabled)
		VALUES($1,$2,$3,$4,$5) ON CONFLICT(id) DO UPDATE SET name=excluded.name,description=excluded.description,
		definition=excluded.definition,enabled=excluded.enabled,updated_at=now()
		RETURNING id,name,description,definition,enabled,created_at,updated_at`, value.ID, value.Name,
		value.Description, value.Definition, value.Enabled).Scan(&value.ID, &value.Name, &value.Description,
		&value.Definition, &value.Enabled, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

func (s *Store) DeletePipeline(ctx context.Context, id string) error {
	command, err := s.pool.Exec(ctx, "DELETE FROM app_pipeline_definitions WHERE id=$1", id)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) AgentSessions(ctx context.Context) ([]domain.AgentSession, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,name,transport,status,metadata,first_seen,last_checkin
		FROM app_agent_sessions ORDER BY last_checkin DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.AgentSession{}
	for rows.Next() {
		var value domain.AgentSession
		if err := rows.Scan(&value.ID, &value.Name, &value.Transport, &value.Status, &value.Metadata,
			&value.FirstSeen, &value.LastCheckin); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) RegisterAgent(ctx context.Context, value domain.AgentSession) (domain.AgentSession, error) {
	if value.ID == "" || value.Name == "" || value.Transport != "simulator" {
		return domain.AgentSession{}, errors.New("only simulator transport is accepted by the management service")
	}
	if len(value.Metadata) == 0 {
		value.Metadata = json.RawMessage(`{}`)
	}
	err := s.pool.QueryRow(ctx, `INSERT INTO app_agent_sessions(id,name,transport,status,metadata)
		VALUES($1,$2,$3,'online',$4) ON CONFLICT(id) DO UPDATE SET name=excluded.name,status='online',
		metadata=excluded.metadata,last_checkin=now()
		RETURNING id,name,transport,status,metadata,first_seen,last_checkin`, value.ID, value.Name,
		value.Transport, value.Metadata).Scan(&value.ID, &value.Name, &value.Transport, &value.Status,
		&value.Metadata, &value.FirstSeen, &value.LastCheckin)
	return value, err
}

func (s *Store) AgentTasks(ctx context.Context, sessionID string) ([]domain.AgentTask, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,session_id,kind,payload,status,result,error,created_at,completed_at
		FROM app_agent_tasks WHERE session_id=$1 ORDER BY created_at DESC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.AgentTask{}
	for rows.Next() {
		var value domain.AgentTask
		if err := rows.Scan(&value.ID, &value.SessionID, &value.Kind, &value.Payload, &value.Status,
			&value.Result, &value.Error, &value.CreatedAt, &value.CompletedAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) CreateAgentTask(ctx context.Context, value domain.AgentTask) (domain.AgentTask, error) {
	if value.Kind != "echo" && value.Kind != "inventory" {
		return domain.AgentTask{}, errors.New("simulator permits only echo and inventory tasks")
	}
	if len(value.Payload) == 0 {
		value.Payload = json.RawMessage(`{}`)
	}
	err := s.pool.QueryRow(ctx, `INSERT INTO app_agent_tasks(id,session_id,kind,payload,status)
		VALUES($1,$2,$3,$4,'queued') RETURNING id,session_id,kind,payload,status,result,error,created_at,completed_at`,
		value.ID, value.SessionID, value.Kind, value.Payload).Scan(&value.ID, &value.SessionID, &value.Kind,
		&value.Payload, &value.Status, &value.Result, &value.Error, &value.CreatedAt, &value.CompletedAt)
	return value, err
}

func (s *Store) DispatchAgentTasks(ctx context.Context, sessionID string) ([]domain.AgentTask, error) {
	if _, err := s.pool.Exec(ctx, `UPDATE app_agent_sessions SET status='online',last_checkin=now() WHERE id=$1`, sessionID); err != nil {
		return nil, err
	}
	if _, err := s.pool.Exec(ctx, `UPDATE app_agent_tasks SET status='dispatched'
		WHERE session_id=$1 AND status='queued'`, sessionID); err != nil {
		return nil, err
	}
	return s.AgentTasks(ctx, sessionID)
}

func (s *Store) CompleteAgentTask(ctx context.Context, sessionID, taskID string, result json.RawMessage, taskErr string) error {
	status := "completed"
	if taskErr != "" {
		status = "failed"
	}
	command, err := s.pool.Exec(ctx, `UPDATE app_agent_tasks SET status=$3,result=$4,error=$5,completed_at=now()
		WHERE id=$1 AND session_id=$2 AND status='dispatched'`, taskID, sessionID, status, result, taskErr)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
