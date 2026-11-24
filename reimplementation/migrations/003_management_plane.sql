CREATE TABLE app_resources (
    kind text NOT NULL,
    resource_key text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'ready',
    spec jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (kind, resource_key)
);

CREATE TABLE app_adapters (
    name text PRIMARY KEY,
    category text NOT NULL,
    mode text NOT NULL CHECK (mode IN ('fixture', 'real')),
    enabled boolean NOT NULL DEFAULT false,
    endpoint text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'unknown',
    last_error text NOT NULL DEFAULT '',
    checked_at timestamptz,
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app_jobs (
    id text PRIMARY KEY,
    kind text NOT NULL,
    status text NOT NULL CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    input jsonb NOT NULL DEFAULT '{}'::jsonb,
    output jsonb,
    error text NOT NULL DEFAULT '',
    attempt integer NOT NULL DEFAULT 0,
    max_attempts integer NOT NULL DEFAULT 1 CHECK (max_attempts BETWEEN 1 AND 10),
    cancel_requested boolean NOT NULL DEFAULT false,
    requested_by text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    completed_at timestamptz
);

CREATE TABLE app_pipeline_definitions (
    id text PRIMARY KEY,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    definition jsonb NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app_agent_sessions (
    id text PRIMARY KEY,
    name text NOT NULL,
    transport text NOT NULL DEFAULT 'simulator',
    status text NOT NULL CHECK (status IN ('online', 'offline', 'archived')),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    first_seen timestamptz NOT NULL DEFAULT now(),
    last_checkin timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app_agent_tasks (
    id text PRIMARY KEY,
    session_id text NOT NULL REFERENCES app_agent_sessions(id) ON DELETE CASCADE,
    kind text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL CHECK (status IN ('queued', 'dispatched', 'completed', 'failed', 'cancelled')),
    result jsonb,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);

CREATE INDEX app_resources_kind_updated_idx ON app_resources(kind, updated_at DESC);
CREATE INDEX app_jobs_status_created_idx ON app_jobs(status, created_at);
CREATE INDEX app_agent_tasks_session_status_idx ON app_agent_tasks(session_id, status, created_at);

INSERT INTO app_adapters(name, category, mode, enabled, endpoint, status) VALUES
    ('postgresql', 'storage', 'real', true, 'internal', 'healthy'),
    ('elasticsearch', 'search', 'fixture', true, 'fixture://elasticsearch', 'healthy'),
    ('smtp', 'messaging', 'fixture', true, 'fixture://smtp', 'healthy'),
    ('acme', 'certificates', 'fixture', true, 'fixture://acme', 'healthy'),
    ('webdav', 'files', 'fixture', true, 'fixture://webdav', 'healthy'),
    ('ai-provider', 'ai', 'fixture', true, 'fixture://ai', 'healthy'),
    ('ssh', 'remote-session', 'fixture', true, 'fixture://ssh', 'healthy'),
    ('tor', 'transport', 'fixture', true, 'fixture://tor', 'healthy'),
    ('winrm', 'remote-session', 'fixture', true, 'fixture://winrm', 'healthy'),
    ('update-service', 'updates', 'fixture', true, 'fixture://updates', 'healthy')
ON CONFLICT (name) DO NOTHING;

INSERT INTO app_pipeline_definitions(id, name, description, definition) VALUES (
    'fixture-safe-transform',
    'Safe transform fixture',
    'Deterministic, non-executable DAG used for compatibility and worker tests.',
    '{"version":1,"nodes":[{"id":"source","type":"input","dependsOn":[]},{"id":"hash","type":"sha256","dependsOn":["source"]},{"id":"encode","type":"base64","dependsOn":["source"]}],"outputs":["hash","encode"]}'::jsonb
) ON CONFLICT (id) DO NOTHING;
