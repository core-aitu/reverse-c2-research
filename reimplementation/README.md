# SpecterInsight clean-room reimplementation

This directory contains the independent Go and TypeScript implementation described by the research plan. It does not contain decompiled C# or redistributed product binaries.

## Implemented management plane

- normalized manifest for all 202 generated REST client operations;
- OpenAPI 3.1 index generated from the pinned `ApiClient.dll` interface;
- compatible PostgreSQL baseline: 36 tables, 10 enums and 81 observed indexes;
- username/password authentication, JWT access tokens and rotating refresh cookies;
- users and roles, documentation/search/index, settings and event history;
- authenticated WebSocket events with cursor replay after reconnect;
- responsive React workflows for all 24 researched screens, including loading, empty, error and permission-aware states;
- `fixture-live-window` path from PostgreSQL through Go and WebSocket to the browser.
- versioned CRUD resources for listeners, certificates, implants, scripts, wordlists, modules, notebooks, tunnels, WinRM, AI, network intelligence, files, updates and licensing;
- persistent jobs with lifecycle, cancellation, attempts and audit events;
- fixture-first ports for PostgreSQL, Elasticsearch, SMTP, ACME, WebDAV, AI, SSH, Tor, WinRM and updates;
- a typed DAG pipeline limited to bounded, non-executable transforms;
- a persistent agent/beacon protocol simulator covering register, check-in, dispatch and results.

The agent gateway is intentionally simulator-only in this service. It does not run a remote shell, load code or generate deployment payloads. Pipeline nodes are an allowlist of deterministic data transforms; shell, PowerShell, network and loader nodes are rejected. These boundaries preserve the orchestration contract while keeping execution isolated and reviewable.

## Run locally

```bash
cd reimplementation
cp .env.example .env
# Replace both secrets in .env before starting.
docker compose --env-file .env up --build
```

Open `http://localhost:3000` and sign in with the configured bootstrap account. The bootstrap password is used only when the account does not already exist.

The data-plane Compose network is internal. The web proxy additionally joins a narrow edge network and is the only service published to the host; PostgreSQL and the Go API are not directly exposed.

Real adapter probes are disabled by default. Set `SPECTER_ENABLE_REAL_ADAPTERS=true` only in an isolated environment after configuring explicit adapter endpoints. Fixture adapters remain deterministic and require no external services.

## Verify

```bash
python3 -m unittest discover -s tests -v
docker run --rm -v "$PWD:/src:Z" -w /src golang:1.27.4-bookworm \
  sh -c 'gofmt -w cmd internal && go test -race ./...'
cd web && pnpm install --frozen-lockfile && pnpm run check && pnpm run build
```

For a full integration check, start Compose, authenticate, open **Events**, and use **Emit fixture-live-window**. The event must appear without refreshing the page. Restarting the server must reconnect and replay events after the browser's last cursor without duplicates.

The same live path is available as a headless smoke test:

```bash
SPECTER_ADMIN_PASSWORD='your configured password' node tests/smoke.mjs
```

## Reproduce generated contracts

Extract `SpecterInsight.ApiClient.dll` and its PDB from the local archive into a temporary directory, decompile there with ILSpy, and run:

```bash
python3 tools/extract_contracts.py \
  --source /tmp/ilspy/SpecterInsight.ApiClient/SpecterInsightClientGenerated.cs \
  --assembly /tmp/ilspy/SpecterInsight.ApiClient.dll \
  --output contracts/manifest.json \
  --openapi contracts/openapi.json

python3 tools/export_schema.py \
  --catalog ../research/validation/stage5/evidence/postgres-catalog.json \
  --output migrations/001_legacy_baseline.sql
```

The scripts fail if the expected 202-operation contract is not recovered. Temporary ILSpy output must not be copied into the repository.
