package store

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/research/specter-reimplementation/internal/domain"
)

const (
	MaxTaskOutputChunk = 32 << 10
	MaxTaskOutput      = 8 << 20
)

func (s *Store) GatewayTask(ctx context.Context, agentID, taskID string) (domain.GatewayTask, error) {
	value, err := scanGatewayTask(s.pool.QueryRow(ctx, `SELECT `+gatewayTaskColumns+` FROM app_agent_gateway_tasks WHERE id=$1 AND agent_id=$2`, taskID, agentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GatewayTask{}, ErrNotFound
	}
	return value, err
}

func (s *Store) StoreTaskOutputChunk(ctx context.Context, agentID string, value domain.TaskOutputChunk) (domain.TaskOutputChunk, bool, error) {
	if value.TaskID == "" || (value.Stream != "stdout" && value.Stream != "stderr") || value.Index < 0 || len(value.Content) > MaxTaskOutputChunk {
		return domain.TaskOutputChunk{}, false, errors.New("invalid task output chunk")
	}
	if value.Content == nil {
		value.Content = []byte{}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.TaskOutputChunk{}, false, err
	}
	defer tx.Rollback(ctx)
	var status string
	if err = tx.QueryRow(ctx, `SELECT status FROM app_agent_gateway_tasks WHERE id=$1 AND agent_id=$2 FOR UPDATE`, value.TaskID, agentID).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
		return domain.TaskOutputChunk{}, false, ErrNotFound
	} else if err != nil {
		return domain.TaskOutputChunk{}, false, err
	}
	if status != domain.TaskRunning && status != domain.TaskLeased {
		return domain.TaskOutputChunk{}, false, errors.New("task is not accepting output")
	}
	var total int64
	if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(octet_length(content)),0) FROM app_agent_task_output_chunks WHERE task_id=$1`, value.TaskID).Scan(&total); err != nil {
		return domain.TaskOutputChunk{}, false, err
	}
	if total+int64(len(value.Content)) > MaxTaskOutput {
		return domain.TaskOutputChunk{}, false, errors.New("task output exceeds limit")
	}
	var stored domain.TaskOutputChunk
	err = tx.QueryRow(ctx, `INSERT INTO app_agent_task_output_chunks(task_id,stream,chunk_index,content,eof,truncated)
		VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(task_id,stream,chunk_index) DO NOTHING
		RETURNING id,task_id,stream,chunk_index,content,eof,truncated,created_at`, value.TaskID, value.Stream, value.Index, value.Content, value.EOF, value.Truncated).
		Scan(&stored.ID, &stored.TaskID, &stored.Stream, &stored.Index, &stored.Content, &stored.EOF, &stored.Truncated, &stored.CreatedAt)
	duplicate := errors.Is(err, pgx.ErrNoRows)
	if duplicate {
		err = tx.QueryRow(ctx, `SELECT id,task_id,stream,chunk_index,content,eof,truncated,created_at FROM app_agent_task_output_chunks
			WHERE task_id=$1 AND stream=$2 AND chunk_index=$3`, value.TaskID, value.Stream, value.Index).
			Scan(&stored.ID, &stored.TaskID, &stored.Stream, &stored.Index, &stored.Content, &stored.EOF, &stored.Truncated, &stored.CreatedAt)
		if err == nil && (subtle.ConstantTimeCompare(stored.Content, value.Content) != 1 || stored.EOF != value.EOF || stored.Truncated != value.Truncated) {
			return domain.TaskOutputChunk{}, false, errors.New("conflicting duplicate output chunk")
		}
	}
	if err != nil {
		return domain.TaskOutputChunk{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.TaskOutputChunk{}, false, err
	}
	decorateOutputChunk(&stored)
	return stored, duplicate, nil
}

func (s *Store) MatchTaskOutputChunk(ctx context.Context, agentID string, value domain.TaskOutputChunk) (domain.TaskOutputChunk, bool, error) {
	var stored domain.TaskOutputChunk
	err := s.pool.QueryRow(ctx, `SELECT c.id,c.task_id,c.stream,c.chunk_index,c.content,c.eof,c.truncated,c.created_at
		FROM app_agent_task_output_chunks c JOIN app_agent_gateway_tasks t ON t.id=c.task_id
		WHERE c.task_id=$1 AND t.agent_id=$2 AND c.stream=$3 AND c.chunk_index=$4`, value.TaskID, agentID, value.Stream, value.Index).
		Scan(&stored.ID, &stored.TaskID, &stored.Stream, &stored.Index, &stored.Content, &stored.EOF, &stored.Truncated, &stored.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TaskOutputChunk{}, false, nil
	}
	if err != nil {
		return domain.TaskOutputChunk{}, false, err
	}
	matched := subtle.ConstantTimeCompare(stored.Content, value.Content) == 1 && stored.EOF == value.EOF && stored.Truncated == value.Truncated
	decorateOutputChunk(&stored)
	return stored, matched, nil
}

func (s *Store) TaskOutputChunks(ctx context.Context, taskID string, after int64) ([]domain.TaskOutputChunk, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,task_id,stream,chunk_index,content,eof,truncated,created_at
		FROM app_agent_task_output_chunks WHERE task_id=$1 AND id>$2 ORDER BY id LIMIT 2000`, taskID, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.TaskOutputChunk{}
	for rows.Next() {
		var value domain.TaskOutputChunk
		if err := rows.Scan(&value.ID, &value.TaskID, &value.Stream, &value.Index, &value.Content, &value.EOF, &value.Truncated, &value.CreatedAt); err != nil {
			return nil, err
		}
		decorateOutputChunk(&value)
		values = append(values, value)
	}
	return values, rows.Err()
}

func decorateOutputChunk(value *domain.TaskOutputChunk) {
	if utf8.Valid(value.Content) {
		value.Encoding = "utf8"
		value.Text = string(value.Content)
		return
	}
	value.Encoding = "base64"
	value.Text = base64.StdEncoding.EncodeToString(value.Content)
}

func joinTaskOutput(values []domain.TaskOutputChunk, stream string) ([]byte, bool, error) {
	var out bytes.Buffer
	next, truncated, sawEOF := 0, false, false
	for _, value := range values {
		if value.Stream != stream {
			continue
		}
		if value.Index != next || sawEOF {
			return nil, false, errors.New("task output chunks are not contiguous")
		}
		next++
		truncated = truncated || value.Truncated
		_, _ = out.Write(value.Content)
		sawEOF = value.EOF
	}
	if next == 0 || !sawEOF {
		return nil, false, errors.New("task output stream is incomplete")
	}
	return out.Bytes(), truncated, nil
}

func (s *Store) FinalizeTaskOutput(ctx context.Context, taskID string) ([]domain.TaskOutputObject, error) {
	if s.objects == nil {
		return nil, errors.New("object storage is not configured")
	}
	existing, err := s.TaskOutputObjects(ctx, taskID)
	if err != nil || len(existing) > 0 {
		return existing, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id,task_id,stream,chunk_index,content,eof,truncated,created_at
		FROM app_agent_task_output_chunks WHERE task_id=$1 ORDER BY stream,chunk_index`, taskID)
	if err != nil {
		return nil, err
	}
	values := []domain.TaskOutputChunk{}
	for rows.Next() {
		var value domain.TaskOutputChunk
		if err = rows.Scan(&value.ID, &value.TaskID, &value.Stream, &value.Index, &value.Content, &value.EOF, &value.Truncated, &value.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		values = append(values, value)
	}
	rows.Close()
	if err = rows.Err(); err != nil || len(values) == 0 {
		return nil, err
	}
	type assembled struct {
		stream, objectID string
		content          []byte
		truncated        bool
	}
	assembledStreams := make([]assembled, 0, 2)
	for _, stream := range []string{"stdout", "stderr"} {
		content, truncated, joinErr := joinTaskOutput(values, stream)
		if joinErr != nil {
			return nil, joinErr
		}
		object, putErr := s.putObjectBytes(ctx, "task-output", taskID+"-"+stream+".log", "application/octet-stream", content)
		if putErr != nil {
			return nil, putErr
		}
		assembledStreams = append(assembledStreams, assembled{stream: stream, objectID: object.ID, content: content, truncated: truncated})
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	for _, value := range assembledStreams {
		if _, err = tx.Exec(ctx, `INSERT INTO app_agent_task_output_objects(task_id,stream,object_id,byte_count,truncated)
			VALUES($1,$2,$3,$4,$5) ON CONFLICT(task_id,stream) DO NOTHING`, taskID, value.stream, value.objectID, len(value.content), value.truncated); err != nil {
			return nil, err
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM app_agent_task_output_chunks WHERE task_id=$1`, taskID); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.TaskOutputObjects(ctx, taskID)
}

func (s *Store) TaskOutputObjects(ctx context.Context, taskID string) ([]domain.TaskOutputObject, error) {
	rows, err := s.pool.Query(ctx, `SELECT x.stream,x.byte_count,x.truncated,
		o.id,o.kind,o.object_key,o.original_name,o.sha256,o.media_type,o.size_bytes,o.created_at
		FROM app_agent_task_output_objects x JOIN app_objects o ON o.id=x.object_id
		WHERE x.task_id=$1 ORDER BY x.stream DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.TaskOutputObject{}
	for rows.Next() {
		var value domain.TaskOutputObject
		if err := rows.Scan(&value.Stream, &value.ByteCount, &value.Truncated,
			&value.Object.ID, &value.Object.Kind, &value.Object.Key, &value.Object.Name, &value.Object.SHA256,
			&value.Object.MediaType, &value.Object.Size, &value.Object.CreatedAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
