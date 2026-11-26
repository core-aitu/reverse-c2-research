# Offline research workflow

These tools operate only on local synthetic event files, generated audit reports and the Markdown research/ADR inventory. They do not contact hosts, use a gateway or alter the running management application. This is an independent research workflow, not a claim of legacy Beacon compatibility.

## 1. Generate two audit reports

From `reimplementation/`, with the repository's Go toolchain:

```sh
go run ./cmd/telemetry-audit -input tests/fixtures/lab-telemetry.json > /tmp/lab-baseline.json
go run ./cmd/telemetry-audit -input tests/fixtures/lab-telemetry-clean.json > /tmp/lab-clean.json
```

The first fixture contains two lifecycle issues, and the second has none. The second fixture is a distinct synthetic scenario, not a correction of recorded events. IDs are intentionally shared to demonstrate comparison. With an older Go installation, the standalone analyzer also runs using `GO111MODULE=off go run .` from `cmd/telemetry-audit`; see [the analyzer documentation](lab-telemetry-audit.md).

## 2. Summarize, compare and view the reports

Python 3.10+ is sufficient; no external package is required.

```sh
python3 tools/research_reports.py summary /tmp/lab-baseline.json
python3 tools/research_reports.py compare /tmp/lab-baseline.json /tmp/lab-clean.json > /tmp/lab-diff.json
python3 tools/research_reports.py html /tmp/lab-baseline.json > /tmp/lab-report.html
```

Open `/tmp/lab-report.html` in a browser. The report is a static HTML document with no scripts, remote assets, server dependencies or live agent state. It contains totals, state and issue counts, session summaries and issue records.

Input reports must use `lab-telemetry-audit/v1`, have unique session IDs and issue event IDs, valid timestamps and consistent counts. The comparison reports additions, removals, changed session records, and issue differences; it does not infer that two captures represent the same real environment. Both reports must be supplied explicitly. JSON output is versioned separately from the management API. Invalid input returns exit code 2; valid input with findings returns 0.

## 3. Verify research documents

```sh
python3 tools/evidence_integrity.py verify docs/research-document-inventory.json
# Create a fresh inventory for a deliberately revised research snapshot:
python3 tools/evidence_integrity.py snapshot > /tmp/research-inventory.json
```

The checked-in baseline contains SHA-256 digests and sizes of nine Markdown files under `research/reports/` and `reimplementation/docs/adr/` (non-recursive). The verifier reports added, missing and changed files and exits 1 for a mismatch, 2 for malformed input, and 0 on a match. It does not read file paths supplied by the manifest. The scope deliberately excludes binaries, raw captures and the locally stored archive. This verifies consistency against a chosen baseline, not authenticity or a digital signature. Review and consciously update the baseline if any covered document changes.

## 4. Tests

```sh
python3 -m unittest discover -s tests -v
go test -race ./cmd/telemetry-audit
```

Python tests include malformed documents, report invariants, comparison, local HTML and integrity checking. Go tests include both synthetic fixtures. The documentation tree tests are still separate (`cd web && pnpm run test:docs`).

No changes to the network listener, agent gateway, task execution or database schemas are made by this workflow. The existing ADR boundaries continue to apply.
