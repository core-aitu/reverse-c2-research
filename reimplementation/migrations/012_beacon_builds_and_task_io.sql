CREATE TABLE app_beacon_releases (
    id text PRIMARY KEY,
    version text NOT NULL CHECK (length(version) BETWEEN 1 AND 128),
    os text NOT NULL CHECK (os IN ('linux','windows','darwin')),
    architecture text NOT NULL CHECK (architecture IN ('amd64','arm64')),
    object_id text NOT NULL REFERENCES app_objects(id) ON DELETE RESTRICT,
    sbom_object_id text REFERENCES app_objects(id) ON DELETE SET NULL,
    checksums_object_id text REFERENCES app_objects(id) ON DELETE SET NULL,
    signature_object_id text REFERENCES app_objects(id) ON DELETE SET NULL,
    signed boolean NOT NULL DEFAULT false,
    development boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(version,os,architecture)
);

CREATE TABLE app_beacon_builds (
    id text PRIMARY KEY,
    release_id text NOT NULL REFERENCES app_beacon_releases(id) ON DELETE RESTRICT,
    created_by text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_bundle_at timestamptz
);

CREATE TABLE app_agent_task_output_chunks (
    id bigserial PRIMARY KEY,
    task_id text NOT NULL REFERENCES app_agent_gateway_tasks(id) ON DELETE CASCADE,
    stream text NOT NULL CHECK (stream IN ('stdout','stderr')),
    chunk_index integer NOT NULL CHECK (chunk_index >= 0),
    content bytea NOT NULL CHECK (octet_length(content) BETWEEN 0 AND 32768),
    eof boolean NOT NULL DEFAULT false,
    truncated boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(task_id,stream,chunk_index)
);

CREATE INDEX app_agent_task_output_replay_idx
    ON app_agent_task_output_chunks(task_id,id);

CREATE TABLE app_agent_task_output_objects (
    task_id text NOT NULL REFERENCES app_agent_gateway_tasks(id) ON DELETE CASCADE,
    stream text NOT NULL CHECK (stream IN ('stdout','stderr')),
    object_id text NOT NULL REFERENCES app_objects(id) ON DELETE RESTRICT,
    byte_count bigint NOT NULL CHECK (byte_count >= 0),
    truncated boolean NOT NULL DEFAULT false,
    PRIMARY KEY(task_id,stream)
);

CREATE TABLE app_agent_file_transfers (
    id text PRIMARY KEY,
    task_id text NOT NULL REFERENCES app_agent_gateway_tasks(id) ON DELETE CASCADE,
    agent_id text NOT NULL REFERENCES app_agent_identities(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 255),
    source_path text NOT NULL CHECK (length(source_path) BETWEEN 1 AND 4096),
    media_type text NOT NULL DEFAULT 'application/octet-stream',
    expected_size bigint NOT NULL CHECK (expected_size BETWEEN 1 AND 1073741824),
    expected_sha256 text NOT NULL CHECK (expected_sha256 ~ '^[0-9a-f]{64}$'),
    received_size bigint NOT NULL DEFAULT 0 CHECK (received_size >= 0),
    next_chunk integer NOT NULL DEFAULT 0 CHECK (next_chunk >= 0),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','complete','failed','aborted')),
    object_id text REFERENCES app_objects(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    UNIQUE(task_id)
);

ALTER TABLE app_agent_gateway_tasks
    DROP CONSTRAINT app_agent_gateway_tasks_kind_check,
    DROP CONSTRAINT app_agent_gateway_tasks_envelope_version_check;

ALTER TABLE app_agent_gateway_tasks
    ADD CONSTRAINT app_agent_gateway_tasks_envelope_version_check CHECK (envelope_version IN (1,2)),
    ADD CONSTRAINT app_agent_gateway_tasks_kind_check CHECK (
        (envelope_version = 1 AND kind IN ('echo','system-info','sleep')) OR
        (envelope_version = 2 AND kind IN ('exec','shell','file-download'))
    );
