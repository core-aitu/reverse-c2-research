CREATE TABLE app_objects (
    id text PRIMARY KEY,
    kind text NOT NULL CHECK (kind ~ '^[a-z0-9][a-z0-9-]{0,63}$'),
    object_key text NOT NULL UNIQUE,
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    size_bytes bigint NOT NULL CHECK (size_bytes > 0),
    media_type text NOT NULL DEFAULT 'application/octet-stream',
    original_name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (kind, sha256)
);

ALTER TABLE app_artifact_contents
    ALTER COLUMN content DROP NOT NULL,
    ADD COLUMN object_id text REFERENCES app_objects(id),
    ADD CONSTRAINT app_artifact_contents_payload_check CHECK (content IS NOT NULL OR object_id IS NOT NULL);

ALTER TABLE app_update_packages
    ALTER COLUMN content DROP NOT NULL,
    ADD COLUMN object_id text REFERENCES app_objects(id),
    ADD CONSTRAINT app_update_packages_payload_check CHECK (content IS NOT NULL OR object_id IS NOT NULL);

CREATE INDEX app_artifact_contents_object_idx ON app_artifact_contents(object_id) WHERE object_id IS NOT NULL;
CREATE INDEX app_update_packages_object_idx ON app_update_packages(object_id) WHERE object_id IS NOT NULL;
