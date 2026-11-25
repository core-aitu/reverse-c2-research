# Offline laboratory telemetry audit

This tool is an independently designed analysis utility, not a legacy Beacon implementation or a wire-protocol compatibility claim. It does not use the management gateway, credentials, network connections, database, or command execution. It reads an explicitly supplied synthetic event file or stdin and writes a JSON report to stdout. The existing execution boundaries in ADR 0001 and ADR 0002 remain unchanged.

## Run

From `reimplementation/`:

```sh
go run ./cmd/telemetry-audit -input tests/fixtures/lab-telemetry.json
go test -race ./cmd/telemetry-audit
```

The command depends only on the Go standard library. If the repository's pinned toolchain cannot be installed, this standalone component can also be tested with a locally installed Go 1.18 or later:

```sh
cd cmd/telemetry-audit
GO111MODULE=off go test -race .
GO111MODULE=off go run . -input ../../tests/fixtures/lab-telemetry.json
```

## Data model

`lab-telemetry/v1` is a local research format, not a SpecterInsight DTO. Each event has an ID, synthetic session ID, RFC3339 timestamp and kind (`opened`, `observed` or `closed`). IDs are unique within the document. Files are bounded to 4 MiB and 20,000 events. Unknown fields, unsupported kinds and invalid records are rejected.

The analyzer orders events by UTC timestamp and preserves input order for equal timestamps. It groups by session ID and reports first and last observations (event timestamps), current state, total observation records and observations within an open session. Missing or inconsistent transitions generate explicit issues without silently changing state. A closed session is terminal; a new synthetic session requires a distinct ID. Invalid input produces an error without a report. Lifecycle issues in otherwise valid input are report data, not CLI failures. The tool makes no claims about actual host reachability, agent timing, delivery or wire-protocol compatibility.

## Next laboratory work

- Add more synthetic scenarios, including overlapping sessions and truncated logs.
- Define a versioned export format for passive research reports, distinct from the legacy API and runtime gateway.
- Add a read-only browser report view after the export contract has been tested.
- Extend compatibility coverage for safe data-management routes and fixture behavior; keep excluded execution and delivery capabilities outside the implementation.
