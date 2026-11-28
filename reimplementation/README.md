# SpecterInsight clean-room reimplementation

This directory contains the independent Go and TypeScript implementation described by the research plan. It does not contain decompiled C# or redistributed product binaries.

## Implemented management plane

- normalized manifest and registered router coverage for all 202 generated REST client operations;
- OpenAPI 3.1 index generated from the pinned `ApiClient.dll` interface;
- compatible PostgreSQL baseline: 36 tables, 10 enums and 81 observed indexes;
- username/password authentication, JWT access tokens and rotating refresh cookies;
- full user/role CRUD with last-admin protection, documentation/search/index, settings and event history;
- authenticated WebSocket events with cursor replay after reconnect;
- responsive React workflows for all 24 researched screens, including loading, empty, error and permission-aware states;
- `fixture-live-window` path from PostgreSQL through Go and WebSocket to the browser;
- versioned CRUD resources for listeners, certificates, implants, scripts, wordlists, modules, notebooks, tunnels, WinRM, AI, network intelligence, files, updates and licensing;
- persistent jobs with lifecycle, cancellation, attempts and audit events;
- a machine-readable capability registry shared by blocked-route responses and the browser matrix;
- fixture-first ports for PostgreSQL, Elasticsearch, SMTP, ACME, WebDAV, AI, SSH, Tor, WinRM and updates, plus persistent compatibility metadata for the remaining safe legacy routes;
- a typed DAG pipeline limited to bounded, non-executable transforms, executed by a one-shot worker without management-plane environment variables;
- legacy simulator routes plus a separate signed, bounded gzip/binary gateway and standalone cross-platform beacon with golden vectors;
- evidence-pinned legacy DTOs and database-backed endpoints for scripts, wordlists, listener configuration, certificate metadata, opaque modules, notebooks and AI definitions.

The original simulator routes remain for compatibility tests. A separate
agent plane now lives under `/api/agent/v1/*`: it uses bounded gzip/binary
frames, one-use enrollment tokens, rotating per-beacon identities, signed
nonce/sequence/timestamp requests, leased versioned task envelopes, explicit
acknowledgements and idempotent results. Envelope v1 retains bounded `echo`,
`system-info` and cancellable `sleep`; envelope v2 adds direct `exec`, explicit
`sh`/`cmd`/PowerShell interpreters and remote-file retrieval with streamed,
bounded output archived in S3. The standalone
`cmd/beacon` process is built as `/app/specter-beacon`; it is not hosted by the
API process. This is an evidence-informed safe protocol, not a claim that the
unobserved legacy wire session has been reproduced byte-for-byte. Protocol
evidence, golden vectors and the restart scenario are in
[`research/validation/gateway-protocol`](../research/validation/gateway-protocol/README.md).
The current pipeline worker accepts deterministic transforms and receives no
management database credentials. These are the delivered baseline, not the
final compatibility target. The superseded boundary is retained in
[`ADR 0001`](docs/adr/0001-safe-compatibility-boundary.md); the active completion
architecture, capability states and enablement gates are defined by
[`ADR 0003`](docs/adr/0003-full-compatibility-execution-plane.md).

The typed catalog groups use the original paths and camel-case JSON representations. Their independently described field catalog is in `contracts/dto-catalog.json`, pinned to the ApiClient artifact SHA-256/MVID and static managed-metadata Evidence. Listener records are configuration only: adding one never binds a network socket. Certificate endpoints store public metadata only and never accept a private key. Uploaded modules are bounded opaque blobs and are never loaded, reflected or executed. Notebook cells are stored as data; the PowerShell session routes return `501` until an isolated worker is configured. AI function definitions use a versioned fixture DSL limited to deterministic `echo`, `concat` and `sha256` operations; unknown fields and execution-shaped operations are rejected.

AI sessions persist the legacy lifecycle, profile snapshot, plan, user messages, approvals and results. The only advertised model is `fixture-deterministic`; creating or resuming a session changes database state but never starts a model process or calls an external provider.

Administration catalogs use typed `ArtifactInfo`, version, update-status and license DTOs. Administrators can ingest bounded artifact content (16 MiB), which is stored with SHA-256 provenance and served by the legacy artifact download route. Update packages are bounded to 32 MiB, rejected unless `X-Content-SHA256` matches, catalogued by a canonical four-part version and served by the legacy client route. Package installation, signature verification and rollback remain worker responsibilities. License keys are retained only as a SHA-256 digest and masked hint and are never externally validated.

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

Production deployments must configure `SPECTER_S3_ENDPOINT`, access key,
secret key and bucket. Legacy `bytea` rows remain readable until the separate
`specter-object-migrate` pass completes, but new artifact, update, release,
task-output and beacon-file content fails closed when object storage is absent.

Real adapter probes are disabled by default. Set `SPECTER_ENABLE_REAL_ADAPTERS=true` only in an isolated environment after configuring explicit adapter endpoints. Fixture adapters remain deterministic and require no external services.

### Beacon quick start

An administrator first creates a single-use token with
`POST /api/gateway/enrollment-tokens` (`{"ttlSeconds":600}`), then starts the
separate process:

```bash
/app/specter-beacon \
  -endpoint https://gateway.lab.example:8443 \
  -ca-bundle /etc/specter/lab-ca.pem \
  -enrollment-token "$ONE_USE_TOKEN" \
  -agent-id lab-agent-01 \
  -interval 5s \
  -state /var/lib/specter-beacon/state.json
```

The **Beacon Builds** screen assembles a one-use ZIP from a published release,
configuration and private CA. Alternatively, queue a version-1 compatibility
task or a version-2 `exec`, `shell` or `file-download` envelope through
`POST /api/gateway/agents/{id}/tasks`; include a unique `idempotencyKey`, UTC
`deadline` and an optional `timeoutMs` (60 seconds by default, five minutes
maximum). Cancel it through
`POST /api/gateway/agents/{id}/tasks/{taskID}/cancel`. Identity controls are
`POST /api/gateway/agents/{id}/{disable|enable|rotate|revoke}`. The returned
token or rotated identity is displayed once and must be stored outside the
management database.

Plain HTTP is rejected. The `-allow-insecure-loopback` flag exists only for an
explicit `localhost`/loopback laboratory and never disables certificate
verification for TLS. Network retries use bounded jitter and exponential
backoff. Agent state, pending results, cursor and sequence are atomically stored
in the configured state file.

The API enables its separate agent listener only when
`SPECTER_AGENT_TLS_CERT` and `SPECTER_AGENT_TLS_KEY` are both configured;
`SPECTER_AGENT_ADDR` defaults to `:8443`. Agent routes reached through the
plain management listener return `426 Upgrade Required`.

Artifact content additionally supports ordered, non-resumable v1 chunk
transfers under `/api/artifact-transfers`. Manifests pin size and SHA-256;
publication happens only after digest verification and an atomic temporary-file
rename. Paths, file size, chunk size/count and concurrent incomplete transfers
are bounded.

Reproducible beacon release inputs are pinned in `go.mod`, `go.sum` and the
Docker base-image digest. `make beacon-matrix sbom checksums VERSION=<version>`
builds the supported matrix and emits deterministic SPDX plus SHA-256 files;
`make sign COSIGN_PRIVATE_KEY=...` signs the checksum manifest without storing
the release key in the repository.

Fixture adapters also expose bounded operation-specific previews under `/api/adapters/{name}/fixture/{operation}`. SMTP and ACME operations return deterministic preview/order IDs without delivery or issuance; remote-session fixtures return empty inventories and never accept command execution.

## Verify

```bash
python3 -m unittest discover -s tests -v
docker build --target test -t specter-reimplementation-test .
cd web && pnpm install --frozen-lockfile && pnpm run check && pnpm test && pnpm run build
```

The Go test target includes a runtime contract-coverage test. It loads the pinned manifest and verifies all 202 unique method/path pairs against the actual `http.ServeMux`; a manifest-only entry is not counted as implemented.

The PostgreSQL integration profile exercises concurrent leasing plus a real
gateway/beacon lifecycle with enrollment, task/result flow and beacon and
gateway restarts. A bounded fuzz target feeds malformed data to every frame
decoder. The multi-hour gate is explicit:

```bash
SPECTER_JWT_SECRET='01234567890123456789012345678901' \
SPECTER_ADMIN_PASSWORD='local-compose-placeholder' \
make soak SOAK_DURATION=4h
```

The soak duration must be at least one minute. It rotates v1 echo, v2 exec,
v2 shell and remote-file retrieval through the complete PostgreSQL,
gateway, beacon and object-store path once per second, while checking dispatch
counts plus goroutine and file-descriptor growth. Passing the short
suite alone does not promote gateway capabilities to `runtime-verified`; that
state requires the full matrix, including a recorded multi-hour soak.

For a full integration check, start Compose, authenticate, open **Events**, and use **Emit fixture-live-window**. The event must appear without refreshing the page. Restarting the server must reconnect and replay events after the browser's last cursor without duplicates.

The same live path is available as a headless smoke test:

```bash
SPECTER_ADMIN_PASSWORD='your configured password' node tests/smoke.mjs
```

The Playwright suite verifies the same running stack through the rendered login,
navigation, scenario, event and logout workflows in Chromium:

```bash
cd web
pnpm exec playwright install chromium
SPECTER_ADMIN_PASSWORD='your configured password' pnpm run test:e2e
```

## Documentation viewer

The web documentation viewer accepts both nested `name`/`contents`/`children` entries observed in the research and the current store's flat slash-separated paths. It supports collapsible navigation, explicit search loading/error/empty states, and reloads without clearing the last successfully loaded document. It aborts superseded requests on dependency changes and unmount. This is a deliberate web UX improvement over the response-order behavior documented in `../research/reports/documentation-ui-semantics.md`. Document content is displayed as escaped text with preserved line breaks, not rendered HTML or full Markdown. The REST contracts and database remain unchanged.

Run the standalone tree/selection tests with `cd web && pnpm run test:docs` on the pinned Node version. On Node 22.16, use `node --experimental-strip-types --test tests/documentation.test.mjs` instead.

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

### Offline research reports and evidence integrity

The standalone laboratory audit can be summarized, compared and rendered to static HTML using `tools/research_reports.py`. `tools/evidence_integrity.py` verifies the SHA-256 baseline for the research reports and ADR documents. Both are local-only Python standard-library utilities. See [the offline workflow](docs/offline-research-workflow.md) for commands, formats and verification limits.

### Research Scenario Engine

The independent **Research → Scenario Engine** runs fixed fictional measurement
quality scenarios and persists results in PostgreSQL. See
[the module documentation](docs/research-scenarios.md) for API, scenarios and
validation. This extension is not part of the legacy compatibility manifest.

### Offline measurement replay

**Research → Measurement Replay** provides a browser-local timeline for two
fictional measurement recordings, with stepwise playback, backward seeking and
quality statistics. It is independent of the gateway and existing scenario API.
See [the replay documentation](docs/measurement-replay.md).

### Research journal

The administrator-only Research Journal displays ordered, persisted events for fictional measurement scenario reports. See [research-journal.md](docs/research-journal.md) for migration, API, and verification details. It is separate from the operational event hub.

### Research plugins

**Research → Research Plugins** offers three built-in browser-local reports for
fixed classroom measurement fixtures, with selection and tabular results. It is
not a general-purpose plugin loader. See [research-plugins.md](docs/research-plugins.md).
