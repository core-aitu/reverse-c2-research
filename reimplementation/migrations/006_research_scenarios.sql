-- Independent research extension; no legacy schema is altered.
CREATE TABLE app_research_scenario_runs (
    id text PRIMARY KEY,
    actor text NOT NULL CHECK (actor <> ''),
    report jsonb NOT NULL CHECK (jsonb_typeof(report) = 'object'),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX app_research_scenario_runs_created_idx
    ON app_research_scenario_runs (created_at DESC, id DESC);
