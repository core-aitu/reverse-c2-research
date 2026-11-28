ALTER TABLE app_objects
    DROP CONSTRAINT app_objects_size_bytes_check,
    ADD CONSTRAINT app_objects_size_bytes_check CHECK (size_bytes >= 0);

ALTER TABLE app_agent_file_transfers
    ADD COLUMN multipart_upload_id text,
    ADD COLUMN multipart_object_key text,
    ADD COLUMN multipart_parts jsonb NOT NULL DEFAULT '[]'::jsonb;

UPDATE app_agent_file_transfers
SET status='aborted',completed_at=COALESCE(completed_at,now())
WHERE status='pending';

ALTER TABLE app_agent_file_transfers
    ADD CONSTRAINT app_agent_file_transfers_multipart_check CHECK (
        status <> 'pending' OR (multipart_upload_id IS NOT NULL AND multipart_object_key IS NOT NULL)
    );
