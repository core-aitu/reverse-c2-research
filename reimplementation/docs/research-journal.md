# Research Journal

The journal stores logical events derived exclusively from the fixed fictional measurement reports produced by `researchscenario.Run`. It is not connected to the operational hub, the gateway, adapters, or remote execution. There is no event ingestion endpoint, polling agent, dispatch or task interface.

- `dataset.checked` starts the logical report sequence.
- `finding.recorded` retains each finding in report order, including repeats.
- `report.completed` records whether findings matched expectations.

Migration `007_research_journal.sql` backfills existing scenario reports. New runs and journal entries are committed together in the same PostgreSQL transaction. The migration assumes the well-formed reports produced by the original Scenario Engine, with a `findings` array and boolean `passed`. Deploy with existing writers stopped, as for other schema migrations. Old reports are not modified. Logical ordinals do not represent actual event timestamps.

Administrator-only `GET /api/research-scenarios/runs/{id}/journal` returns the ordered entries for one existing run, 404 for a missing run. No external source may publish events. The existing history endpoint selects the latest 50 runs. Open **Research → Research Journal**, select a run and step through the entries. All data are read-only in the viewer.

## Verification

Run `go test -race ./internal/researchjournal ./internal/researchscenario ./internal/httpapi ./internal/store` and the existing frontend check. With PostgreSQL running, verify that a scenario created before migration is backfilled, a new scenario has all journal rows, and a missing run returns 404. The migration and full application must be verified against a real database before deployment.
