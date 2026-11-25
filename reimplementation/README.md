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
- `fixture-live-window` path from PostgreSQL through Go and WebSocket to the browser;
- versioned CRUD resources for listeners, certificates, implants, scripts, wordlists, modules, notebooks, tunnels, WinRM, AI, network intelligence, files, updates and licensing;
- persistent jobs with lifecycle, cancellation, attempts and audit events;
- fixture-first ports for PostgreSQL, Elasticsearch, SMTP, ACME, WebDAV, AI, SSH, Tor, WinRM and updates;
- a typed DAG pipeline limited to bounded, non-executable transforms, executed by a one-shot worker without management-plane environment variables;
- a persistent agent/beacon protocol simulator covering register, check-in, dispatch and results, plus a bounded internal `SIR1` framing contract with golden vectors;
- evidence-pinned legacy DTOs and database-backed endpoints for scripts, wordlists, listener configuration, certificate metadata, opaque modules, notebooks and AI definitions.

The agent gateway is intentionally simulator-only in this service. It does not run a remote shell, load code or generate deployment payloads. `SIR1` is an internal simulator framing contract, not a claim of compatibility with an unverified legacy wire format. Pipeline nodes are an allowlist of deterministic data transforms; shell, PowerShell, network and loader nodes are rejected. Jobs are passed over a versioned stdin/stdout contract to a one-shot worker process whose environment omits the management database URL and credentials. These boundaries preserve the orchestration contract while keeping execution isolated and reviewable.

The typed catalog groups use the original paths and camel-case JSON representations. Their independently described field catalog is in `contracts/dto-catalog.json`, pinned to the ApiClient artifact SHA-256/MVID and static managed-metadata Evidence. Listener records are configuration only: adding one never binds a network socket. Certificate endpoints store public metadata only and never accept a private key. Uploaded modules are bounded opaque blobs and are never loaded, reflected or executed. Notebook cells are stored as data; the PowerShell session routes return `501` until an isolated worker is configured. AI function definitions use a versioned fixture DSL limited to deterministic `echo`, `concat` and `sha256` operations; unknown fields and execution-shaped operations are rejected.

AI sessions persist the legacy lifecycle, profile snapshot, plan, user messages, approvals and results. The only advertised model is `fixture-deterministic`; creating or resuming a session changes database state but never starts a model process or calls an external provider.

Administration catalogs use typed `ArtifactInfo`, version, update-status and license DTOs. Artifact downloads return `404` when only metadata exists; fixture update checks never download or install packages. License keys are retained only as a SHA-256 digest and masked hint and are never externally validated.

Network Intelligence uses the legacy normalized host, IP-address and open-port tables with typed DTOs, validated IP/CIDR input and cascading host deletion. The web map reads these exact endpoints instead of the generic resource registry.

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

Fixture adapters also expose bounded operation-specific previews under `/api/adapters/{name}/fixture/{operation}`. SMTP and ACME operations return deterministic preview/order IDs without delivery or issuance; remote-session fixtures return empty inventories and never accept command execution.

## Verify

```bash
python3 -m unittest discover -s tests -v
docker build --target test -t specter-reimplementation-test .
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
