ALTER TABLE app_agent_gateway_tasks
    DROP CONSTRAINT app_agent_gateway_tasks_kind_check,
    DROP CONSTRAINT app_agent_gateway_tasks_status_check;

ALTER TABLE app_agent_gateway_tasks
    ADD COLUMN envelope_version smallint NOT NULL DEFAULT 1 CHECK (envelope_version = 1),
    ADD COLUMN idempotency_key text,
    ADD COLUMN deadline timestamptz,
    ADD COLUMN timeout_ms integer NOT NULL DEFAULT 30000 CHECK (timeout_ms BETWEEN 100 AND 300000),
    ADD COLUMN cancel_requested_at timestamptz,
    ADD COLUMN started_at timestamptz,
    ADD COLUMN error_retryable boolean NOT NULL DEFAULT false,
    ADD COLUMN submission_digest text CHECK (submission_digest IS NULL OR submission_digest ~ '^[0-9a-f]{64}$');

ALTER TABLE app_agent_identities
    ADD COLUMN previous_identity_hash text,
    ADD COLUMN previous_identity_valid_until timestamptz;

UPDATE app_agent_gateway_tasks
SET idempotency_key = id,
    deadline = created_at + interval '5 minutes'
WHERE idempotency_key IS NULL OR deadline IS NULL;

ALTER TABLE app_agent_gateway_tasks
    ALTER COLUMN idempotency_key SET NOT NULL,
    ALTER COLUMN deadline SET NOT NULL,
    ADD CONSTRAINT app_agent_gateway_tasks_kind_check CHECK (kind IN ('echo','system-info','sleep')),
    ADD CONSTRAINT app_agent_gateway_tasks_status_check CHECK (status IN ('queued','leased','running','completed','failed','cancelled')),
    ADD CONSTRAINT app_agent_gateway_tasks_idempotency_unique UNIQUE (agent_id,idempotency_key);

CREATE INDEX app_agent_gateway_tasks_deadline_idx
    ON app_agent_gateway_tasks(agent_id,deadline)
    WHERE status IN ('queued','leased','running');

CREATE TABLE app_agent_request_nonces (
    agent_id text NOT NULL REFERENCES app_agent_identities(id) ON DELETE CASCADE,
    nonce text NOT NULL CHECK (length(nonce) BETWEEN 16 AND 128),
    sequence bigint NOT NULL CHECK (sequence > 0),
    observed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id,nonce)
);

CREATE INDEX app_agent_request_nonces_expiry_idx ON app_agent_request_nonces(observed_at);
