# ADR 0003: Complete compatibility through isolated execution planes

- Status: accepted
- Date: 2026-11-26
- Scope: completion target for the SpecterInsight 7.0.0 clean-room reimplementation
- Supersedes: ADR 0001 safe compatibility boundary and the final-state portion of ADR 0002

## Context

The first delivery intentionally stopped at an authenticated management plane,
persistent metadata, deterministic fixtures, safe transforms and an internal
agent simulator. That boundary made the reconstructed contracts testable, but
it left some registered legacy routes with fixture semantics or HTTP `501`.

The project goal is now to finish every evidence-backed capability rather than
treat simulator-only behavior as the final product. Route registration alone is
not completion. A capability is complete only when its domain behavior,
persistence, isolated runtime, audit trail, UI and repeatable integration tests
agree with the recovered evidence.

## Decision

The control plane and execution planes remain separate deployments, but the
execution planes are now required deliverables rather than hypothetical future
work.

1. The Go API remains the authenticated control plane. It validates requests,
   applies RBAC, persists lifecycle state, issues bounded jobs and records audit
   events. It never loads supplied code or holds a remote interactive process.
2. Runtime behavior is implemented by capability-specific workers using a
   versioned stdin/stdout or mutually authenticated RPC contract. Workers receive
   one scoped job, short-lived credentials and an explicit resource budget. They
   receive no management database URL, JWT signing key or refresh-token state.
3. Workers are split by trust and platform boundary:
   - remote-session workers for SSH, WinRM and PowerShell session lifecycles;
   - file-transfer workers for upload, download, WebDAV and cached artifacts;
   - build workers for evidence-backed pipeline, loader and package operations;
   - service workers for ACME, SMTP, Tor, tunnels and update retrieval;
   - AI workers for configured model providers and MCP transport;
   - gateway workers for the verified agent framing and task lifecycle.
4. A capability is disabled until its worker advertises a pinned contract
   version and passes health, isolation and golden-vector checks. Enablement is
   explicit per capability and per environment; fixture mode remains available
   for deterministic tests.
5. Secrets are referenced by opaque IDs. The control plane never persists a
   recoverable password, private key or provider token in compatibility JSON.
   A worker resolves only the secret IDs attached to its current job, and the
   resolution expires with that job.
6. Filesystem operations use a configured root, descriptor-relative access
   where supported, canonical containment checks and bounded sizes. Transfers
   use content hashes and atomic finalization.
7. Network destinations are normalized and checked against an administrator
   policy before dispatch. Redirects, DNS rebinding results and proxy targets
   are re-evaluated by the worker.
8. Existing security corrections remain intentional compatibility differences:
   administrative routes stay authenticated, WebSockets keep single-use tickets
   and Origin enforcement, and unsafe anonymous behavior is not reproduced.
9. No undocumented wire behavior is invented. When evidence is insufficient,
   the capability remains explicitly `evidence-blocked` and references the
   corresponding entry in `research/validation/gaps.md`.

## Capability state model

Every legacy capability is tracked in exactly one of these states:

- `registered`: method and path exist, but domain behavior is absent;
- `fixture`: deterministic lifecycle or preview with no external side effect;
- `implemented`: a real isolated worker performs the operation;
- `runtime-verified`: the real path passed a repeatable integration scenario
  with correlated control-plane, worker, network and persistence evidence;
- `evidence-blocked`: implementation would require inventing an unknown protocol
  or format.

Only `runtime-verified` counts as complete. `evidence-blocked` is an explicit
research dependency, not an implementation success.

## Completion matrix

| Capability group | Current baseline | Required completion evidence |
|---|---|---|
| Authentication, users, settings, documentation and event replay | implemented and integration-tested | browser and API regression suite remains green |
| Typed catalogs, network intelligence, jobs and research modules | implemented and integration-tested | CRUD, migration and browser coverage remains green |
| Listener and tunnel lifecycles | fixture | isolated bind/forward tests, cancellation and port cleanup |
| SSH, WinRM and PowerShell sessions | fixture or `501` | platform worker, command/task lifecycle, transfer tests and teardown |
| ACME, SMTP, WebDAV and Tor | fixture/probe | isolated service integration with recorded success and failure paths |
| Artifact content | runtime-verified bounded SHA-256 store and legacy download | keep API, browser and repeatability tests green |
| Update package staging and download | runtime-verified SHA-256 catalog and legacy download | keep digest rejection and content round-trip tests green |
| Update installation | registered only | signing policy, isolated installer, health confirmation and rollback tests |
| AI providers and MCP | deterministic fixture | configured provider worker, MCP handshake, approvals and cancellation |
| Agent gateway and framing | internal simulator | evidence-pinned framing vectors and isolated session interoperability |
| Payload, loader and build pipeline | bounded safe DAG or `501` | evidence-pinned one-shot build workers, reproducibility and artifact provenance |

## Enablement gates

A route may stop returning `CAPABILITY_NOT_IMPLEMENTED` only when all applicable
gates pass:

1. request and response DTOs are pinned to recovered evidence;
2. the worker protocol has golden vectors and rejects unknown versions;
3. authentication, authorization and secret redaction tests pass;
4. CPU, memory, wall-time, output-size and concurrency limits are enforced;
5. cancellation terminates the worker and releases files, sockets and children;
6. audit events correlate request, job, worker and result identifiers;
7. success, failure, timeout and restart scenarios are repeatable in CI or the
   documented platform laboratory;
8. the management API process still has no execution implementation or worker
   credentials in its address space.

## Verification and definition of done

The project is fully complete when:

- all 202 legacy method/path pairs have semantic contract tests, not merely
  router-coverage tests;
- every non-`evidence-blocked` capability is `runtime-verified`;
- every `evidence-blocked` entry names the missing artifact and has no fabricated
  fallback presented as compatibility;
- PostgreSQL migrations, Go race tests, Python evidence tests, TypeScript tests,
  the API/WebSocket smoke test and Chromium end-to-end suite all pass;
- an isolation test proves that workers cannot read management credentials or
  connect directly to the management database;
- the completion matrix is generated from the same capability registry used by
  the router and fails CI when documentation and runtime state disagree.

## Consequences

The earlier fixture behavior remains useful for tests, but it is no longer the
definition of completion. The new target is larger and platform-dependent. It
requires separate worker images or hosts, explicit operator configuration and
evidence for each legacy protocol. Compatibility claims become narrower and
more defensible because an unverified behavior is reported as blocked instead
of being silently approximated.
