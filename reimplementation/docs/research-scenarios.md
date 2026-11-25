# Research Scenario Engine (independent extension)

This feature is a new research quality-assurance module, not a claim of
legacy SpecterInsight compatibility. It is separate from the legacy 202-route
manifest, agent gateway, worker, listener, event stream and payload pipeline.
It performs no network or filesystem operations in the engine, accepts no
host, commands, scripts, user-defined steps or executable payloads, and uses
only five fixed fictional measurement datasets.

## Run it

Start the existing Compose stack as described in the main README. Migration
`006_research_scenarios.sql` is applied automatically at server startup.
Sign in as an administrator and open **Research → Scenario Engine**.
Choose a preset, select **Run scenario**, and inspect the stored report.

The fictional instrument emits sequence numbers 1–3 and measurements in
[0,100]. Each preset defines its expected finding codes. A run passes if
its actual finding codes (including multiplicity) match the expectation.
A PASS for a deliberately malformed dataset means the validation worked.

| Preset | Expected findings |
| --- | --- |
| Complete series | None |
| Duplicate observation | DUPLICATE |
| Missing observation | MISSING |
| Out-of-order observations | OUT_OF_ORDER |
| Multiple quality issues | DUPLICATE, MISSING, OUT_OF_RANGE |

## API (admin bearer token required)

| Method | Path | Description |
| --- | --- | --- |
| GET | `/api/research-scenarios` | Fixed preset catalog |
| POST | `/api/research-scenarios/runs` | Execute preset; body `{"scenarioId":"mixed-errors"}`; returns 201 |
| GET | `/api/research-scenarios/runs` | Latest 50 saved reports, newest first |

Each run stores a generated identifier, authenticated actor, database
creation timestamp, engine version, original observations, actual findings,
expected finding codes and pass/fail. History is shared among administrators;
older runs remain stored but are not currently displayed. No automatic
retention or background execution is provided. Requests are bounded to 4096
bytes; unknown presets, unknown properties and trailing JSON are rejected.

## Testing

- `go test -race ./internal/researchscenario` checks the deterministic engine.
- `go test ./internal/httpapi -run TestResearchScenarioRouteBoundaries` checks routing, authentication, authorization and invalid requests.
- `node tests/smoke.mjs` (with the Compose stack and admin credentials) checks persisted results via HTTP, alongside existing smoke tests.
- `pnpm run check` in `web/` checks the UI together with the existing application.
