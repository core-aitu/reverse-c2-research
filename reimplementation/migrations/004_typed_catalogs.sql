CREATE TABLE app_certificates (
    name text PRIMARY KEY,
    country text NOT NULL DEFAULT '',
    state text NOT NULL DEFAULT '',
    locality text NOT NULL DEFAULT '',
    organization text NOT NULL DEFAULT '',
    organizational_unit text NOT NULL DEFAULT '',
    common_name text NOT NULL DEFAULT '',
    valid_from timestamptz NOT NULL,
    valid_until timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (valid_until > valid_from)
);

CREATE TABLE app_modules (
    name text PRIMARY KEY,
    version text NOT NULL DEFAULT '',
    runtime_version text NOT NULL DEFAULT '',
    cmdlets text[] NOT NULL DEFAULT '{}',
    content_sha256 text NOT NULL DEFAULT '',
    content bytea,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app_contract_provenance (
    contract_name text PRIMARY KEY,
    artifact_sha256 text NOT NULL,
    artifact_mvid text NOT NULL,
    evidence_ids text[] NOT NULL,
    observation_kind text NOT NULL,
    recorded_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO app_contract_provenance(contract_name, artifact_sha256, artifact_mvid, evidence_ids, observation_kind)
VALUES (
    'management-catalog-dtos',
    '5c658f03289e5804a692ee66ca7beff3883631f4a3d81b301bdade0e7fa45422',
    '25208fa6-b62b-46bd-b864-eea28025b41e',
    ARRAY[
        'ev_92b4c01be83d404ed4a30833d18f8c0e808a857c7161bbe4122f52e6ebf73a84',
        'ev_ea230181ffcd9ba9f70cc982feb949a41b39fd46b143b87671526a44564ec76e'
    ],
    'static-managed-metadata'
) ON CONFLICT (contract_name) DO NOTHING;
