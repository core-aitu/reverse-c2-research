CREATE TABLE app_agent_enrollment_tokens (
    id text PRIMARY KEY,
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_by text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app_agent_identities (
    id text PRIMARY KEY,
    name text NOT NULL,
    identity_hash text NOT NULL UNIQUE,
    generation integer NOT NULL DEFAULT 1,
    enabled boolean NOT NULL DEFAULT true,
    revoked_at timestamptz,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    last_sequence bigint NOT NULL DEFAULT 0,
    last_cursor bigint NOT NULL DEFAULT 0,
    first_seen timestamptz NOT NULL DEFAULT now(),
    last_checkin timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app_agent_gateway_tasks (
    cursor bigserial UNIQUE,
    id text PRIMARY KEY,
    agent_id text NOT NULL REFERENCES app_agent_identities(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind = 'echo'),
    payload jsonb NOT NULL,
    status text NOT NULL CHECK (status IN ('queued','leased','completed','failed','cancelled')),
    lease_until timestamptz,
    dispatch_count integer NOT NULL DEFAULT 0,
    acknowledged_at timestamptz,
    submission_id text UNIQUE,
    result jsonb,
    error_code text NOT NULL DEFAULT '',
    error_message text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);

CREATE INDEX app_agent_gateway_tasks_dispatch_idx
    ON app_agent_gateway_tasks(agent_id, status, lease_until, cursor);
CREATE INDEX app_agent_enrollment_tokens_expiry_idx
    ON app_agent_enrollment_tokens(expires_at) WHERE used_at IS NULL;
