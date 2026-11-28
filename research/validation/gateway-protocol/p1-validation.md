# Safe gateway P1 validation

This note describes clean-room safety behavior, not newly observed legacy wire
behavior.

## Task lifecycle

Task envelopes carry version 1, task and idempotency identifiers, cursor, an
allowlisted type, bounded JSON payload, UTC deadline, wall timeout and cancel
flag. PostgreSQL constrains the lifecycle to `queued`, `leased`, `running` and
terminal `completed`, `failed` or `cancelled` states. Expired leases may return
to `leased`; terminal states cannot transition back. The `(agent_id,
idempotency_key)` pair is unique. A concurrent PostgreSQL test uses
`FOR UPDATE SKIP LOCKED` to prove exclusive leasing and proves that duplicate
submission IDs are accepted only as an idempotent replay.

## Resilience and transport

The beacon persists its next sequence before each request and persists cursor
plus pending result before delivery. A completed cursor is not executed again
after restart. Pending results are capped at 64. After 24 hours a retained
payload is replaced by a retryable `stale_result` error and delivered rather
than silently discarded.

Only verified TLS 1.3+ is enabled outside an explicitly opted-in loopback lab.
A bounded PEM CA bundle may extend system roots; there is no skip-verification
mode. DNS/connect, TLS, response-header and whole-request deadlines are bounded.
The server exposes agent routes on a separate TLS listener and rejects those
routes on the plain management listener.
Certificate/authentication/protocol errors are permanent; timeouts, 408, 425,
429 and 5xx responses receive exponential backoff with bounded jitter.

Every authenticated request is HMAC-SHA256 signed over method, route, protocol
version, timestamp, nonce, sequence and body digest. The server enforces a
five-minute clock-skew window, unique nonce and monotonic sequence. Identity
rotation keeps the prior hash valid for five minutes, permitting safe recovery
when the rotation response is lost. The server persists only identity hashes;
the beacon keeps its current identity in its mode-0600 local state file.

## Worker boundary

The worker has an explicit handler map for `echo`, `system-info` and `sleep`.
It rejects unknown JSON fields and task types, bounds parallelism, payload,
result and wall time, creates a private temporary directory for every task and
removes it on success, error, timeout, cancellation or panic. It starts no
process and receives no gateway identity. Consequently there is no child tree
to kill in P1. Script/shell/interpreter execution remains `evidence-blocked`.

## Artifact transfer and builds

Artifact transfer v1 is ordered and explicitly non-resumable. It bounds file,
chunk count, chunk size and pending transfers; rejects absolute/traversal paths;
writes a private `.part` file; verifies size and SHA-256; atomically renames to
`.ready`; and publishes through the existing artifact content store before
deleting temporary content. Upload, chunk, completion, abort, manifest and
download events are audited.

The Go toolchain, module graph and Docker base digest are pinned. Release builds
use `CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false`, an empty build ID and an
explicit build/protocol version. Make targets generate Linux, Windows and
macOS artifacts, deterministic SPDX, SHA-256 and a cosign signature. Signing
requires an external key and is therefore not claimed as executed in this
repository snapshot.
