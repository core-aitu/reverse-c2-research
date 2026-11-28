CREATE TABLE app_artifact_contents (
    artifact_id integer PRIMARY KEY REFERENCES artifacts(id) ON DELETE CASCADE,
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    media_type text NOT NULL DEFAULT 'application/octet-stream',
    content bytea NOT NULL CHECK (octet_length(content) BETWEEN 1 AND 16777216),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app_update_packages (
    version_key text PRIMARY KEY,
    major integer NOT NULL CHECK (major BETWEEN 0 AND 65535),
    minor integer NOT NULL CHECK (minor BETWEEN 0 AND 65535),
    build integer NOT NULL CHECK (build BETWEEN 0 AND 65535),
    revision integer NOT NULL CHECK (revision BETWEEN 0 AND 65535),
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    media_type text NOT NULL DEFAULT 'application/octet-stream',
    content bytea NOT NULL CHECK (octet_length(content) BETWEEN 1 AND 33554432),
    active boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX app_update_packages_one_active
    ON app_update_packages(active) WHERE active = true;
