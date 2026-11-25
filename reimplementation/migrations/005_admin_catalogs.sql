CREATE TABLE app_license_state (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    email text NOT NULL DEFAULT '',
    key_digest text NOT NULL DEFAULT '',
    key_hint text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app_update_state (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    major integer NOT NULL DEFAULT 7,
    minor integer NOT NULL DEFAULT 0,
    build integer NOT NULL DEFAULT 0,
    revision integer NOT NULL DEFAULT 0,
    status text NOT NULL DEFAULT 'Idle',
    status_description text NOT NULL DEFAULT 'fixture update service; no package downloaded',
    last_update_result text NOT NULL DEFAULT 'None',
    progress integer NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO app_license_state(singleton) VALUES(true) ON CONFLICT(singleton) DO NOTHING;
INSERT INTO app_update_state(singleton) VALUES(true) ON CONFLICT(singleton) DO NOTHING;

INSERT INTO app_contract_provenance(contract_name, artifact_sha256, artifact_mvid, evidence_ids, observation_kind)
VALUES (
    'administration-catalog-dtos',
    '5c658f03289e5804a692ee66ca7beff3883631f4a3d81b301bdade0e7fa45422',
    '25208fa6-b62b-46bd-b864-eea28025b41e',
    ARRAY[
        'ev_9e0b378f36a24eea63826e10d36afa0e4559454dde7aa2d265c8f24609478ed8',
        'ev_11e8359481594eb6c88c9b69b7ab12f453e31fc932d4dd9ee7d9fe8d112e4da2'
    ],
    'static-managed-metadata'
) ON CONFLICT (contract_name) DO NOTHING;
