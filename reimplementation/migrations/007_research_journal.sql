-- Fixed research scenario results only. No operational event ingestion or dispatch.
CREATE TABLE app_research_journal (
    run_id text NOT NULL REFERENCES app_research_scenario_runs(id) ON DELETE CASCADE,
    ordinal integer NOT NULL CHECK (ordinal > 0),
    kind text NOT NULL,
    code text,
    sequence integer,
    passed boolean,
    PRIMARY KEY (run_id, ordinal),
    CHECK (
        (kind = 'dataset.checked' AND ordinal = 1 AND code IS NULL AND sequence IS NULL AND passed IS NULL) OR
        (kind = 'finding.recorded' AND ordinal > 1 AND code IS NOT NULL AND sequence IS NOT NULL AND passed IS NULL) OR
        (kind = 'report.completed' AND ordinal > 1 AND code IS NULL AND sequence IS NULL AND passed IS NOT NULL)
    )
);

-- Backfill existing fictional scenario reports in their original finding order.
INSERT INTO app_research_journal (run_id, ordinal, kind)
SELECT id, 1, 'dataset.checked' FROM app_research_scenario_runs;

INSERT INTO app_research_journal (run_id, ordinal, kind, code, sequence)
SELECT r.id, (f.position + 1)::integer, 'finding.recorded', f.value->>'code', (f.value->>'sequence')::integer
FROM app_research_scenario_runs AS r
CROSS JOIN LATERAL jsonb_array_elements(r.report->'findings') WITH ORDINALITY AS f(value, position);

INSERT INTO app_research_journal (run_id, ordinal, kind, passed)
SELECT id, jsonb_array_length(report->'findings') + 2, 'report.completed', (report->>'passed')::boolean
FROM app_research_scenario_runs;
