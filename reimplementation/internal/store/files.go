package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/research/specter-reimplementation/internal/domain"
	"github.com/research/specter-reimplementation/internal/objectstore"
)

func (s *Store) CreateAgentFileTransfer(ctx context.Context, value domain.AgentFileTransfer) (domain.AgentFileTransfer, error) {
	if value.ID == "" || value.TaskID == "" || value.AgentID == "" || value.Name == "" || value.ExpectedSize < 0 || value.ExpectedSize > 1<<30 || len(value.ExpectedSHA256) != 64 || value.MultipartUploadID == "" || value.MultipartObjectKey == "" {
		return domain.AgentFileTransfer{}, errors.New("invalid agent file transfer")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.AgentFileTransfer{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, value.AgentID); err != nil {
		return domain.AgentFileTransfer{}, err
	}
	var active int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM app_agent_file_transfers WHERE agent_id=$1 AND status='pending'`, value.AgentID).Scan(&active); err != nil {
		return domain.AgentFileTransfer{}, err
	}
	if active >= 2 {
		return domain.AgentFileTransfer{}, errors.New("agent already has two active file transfers")
	}
	var kind, status string
	if err = tx.QueryRow(ctx, `SELECT kind,status FROM app_agent_gateway_tasks WHERE id=$1 AND agent_id=$2`, value.TaskID, value.AgentID).Scan(&kind, &status); err != nil {
		return domain.AgentFileTransfer{}, err
	}
	if kind != "file-download" || (status != domain.TaskRunning && status != domain.TaskLeased) {
		return domain.AgentFileTransfer{}, errors.New("task is not accepting a file")
	}
	if value.MediaType == "" {
		value.MediaType = "application/octet-stream"
	}
	_, err = tx.Exec(ctx, `INSERT INTO app_agent_file_transfers(id,task_id,agent_id,name,source_path,media_type,expected_size,expected_sha256,multipart_upload_id,multipart_object_key)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, value.ID, value.TaskID, value.AgentID, value.Name, value.SourcePath, value.MediaType, value.ExpectedSize, value.ExpectedSHA256, value.MultipartUploadID, value.MultipartObjectKey)
	if err != nil {
		return domain.AgentFileTransfer{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.AgentFileTransfer{}, err
	}
	return s.AgentFileTransfer(ctx, value.ID, value.AgentID)
}

func (s *Store) AgentFileTransfer(ctx context.Context, id, agentID string) (domain.AgentFileTransfer, error) {
	var value domain.AgentFileTransfer
	var partsJSON []byte
	err := s.pool.QueryRow(ctx, `SELECT id,task_id,agent_id,name,source_path,media_type,expected_size,expected_sha256,received_size,next_chunk,status,multipart_upload_id,multipart_object_key,multipart_parts,created_at,completed_at
		FROM app_agent_file_transfers WHERE id=$1 AND agent_id=$2`, id, agentID).
		Scan(&value.ID, &value.TaskID, &value.AgentID, &value.Name, &value.SourcePath, &value.MediaType, &value.ExpectedSize, &value.ExpectedSHA256,
			&value.ReceivedSize, &value.NextChunk, &value.Status, &value.MultipartUploadID, &value.MultipartObjectKey, &partsJSON, &value.CreatedAt, &value.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AgentFileTransfer{}, ErrNotFound
	}
	if err == nil {
		err = json.Unmarshal(partsJSON, &value.MultipartParts)
	}
	return value, err
}

func (s *Store) AdvanceAgentFileTransfer(ctx context.Context, id, agentID string, index int, received int64, part objectstore.MultipartPart) error {
	partJSON, _ := json.Marshal(domain.MultipartPart{Number: part.Number, ETag: part.ETag, Size: part.Size})
	command, err := s.pool.Exec(ctx, `UPDATE app_agent_file_transfers SET received_size=$4,next_chunk=$3+1,multipart_parts=multipart_parts || $5::jsonb
		WHERE id=$1 AND agent_id=$2 AND status='pending' AND next_chunk=$3 AND received_size<$4`, id, agentID, index, received, partJSON)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) CompleteAgentFileTransfer(ctx context.Context, id, agentID, objectID string, at time.Time) error {
	command, err := s.pool.Exec(ctx, `UPDATE app_agent_file_transfers SET status='complete',object_id=$3,completed_at=$4
		WHERE id=$1 AND agent_id=$2 AND status='pending' AND received_size=expected_size`, id, agentID, objectID, at)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) FailAgentFileTransfer(ctx context.Context, id, agentID, status string) error {
	if status != "failed" && status != "aborted" {
		return errors.New("invalid file transfer terminal status")
	}
	command, err := s.pool.Exec(ctx, `UPDATE app_agent_file_transfers SET status=$3,completed_at=now()
		WHERE id=$1 AND agent_id=$2 AND status='pending'`, id, agentID, status)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
