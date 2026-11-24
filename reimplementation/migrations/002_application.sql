CREATE TABLE app_account_roles (
    account_id integer NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('admin', 'operator', 'viewer')),
    PRIMARY KEY (account_id, role)
);

CREATE TABLE app_settings (
    key text PRIMARY KEY,
    value text NOT NULL,
    updated_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX refreshtokens_token_key ON refreshtokens(token);
CREATE INDEX event_timestamp_idx ON event(timestamp DESC, id DESC);
CREATE INDEX documentationchunks_search_idx
    ON documentationchunks USING gin ((title || ' ' || content) gin_trgm_ops);

INSERT INTO app_settings(key, value) VALUES
    ('server.name', 'Specter Research Node'),
    ('events.retentionDays', '30'),
    ('documentation.searchProvider', 'postgres-trigram')
ON CONFLICT (key) DO NOTHING;
