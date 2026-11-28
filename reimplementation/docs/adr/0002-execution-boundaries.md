# ADR 0002: Execution boundaries for agents and payload pipelines

## Status

Superseded as the final target by
[ADR 0003](0003-full-compatibility-execution-plane.md). The restrictions in this
document remain the active implementation boundary for capabilities that have
not yet passed ADR 0003's enablement gates.

## Context

The observed product contract includes agent tasking, remote-session commands, loaders and payload build pipelines. Reproducing those operations inside the web/API process would couple the management database to arbitrary code execution and would copy unsafe behavior that the clean-room plan explicitly excludes.

## Decision

- The management service retains the protocol simulator and now also exposes a
  bounded authenticated gateway. Execution remains in a separate bounded
  beacon worker; the API owns enrollment, leases, persistence and audit only.
- The safe worker allowlists only `echo`, `system-info` and `sleep`, creates a
  private temporary directory per task, enforces deadline/wall-time/output and
  concurrency bounds, and always removes the directory. These handlers never
  start child processes, so process-tree termination is intentionally dormant
  until an evidence-approved process task exists.
- Simulator tasks are allowlisted to `echo` and `inventory`. They never spawn a process or connect to a remote host.
- Payload orchestration is represented as a versioned DAG. The management worker accepts only `input`, `literal`, `concat`, `base64`, `base64decode`, `sha256`, `uppercase` and `lowercase` nodes.
- Inputs, outputs, node counts and execution time are bounded. Shell, PowerShell, loader, filesystem and network nodes are rejected during validation.
- External adapters start in deterministic fixture mode. Real probes require `SPECTER_ENABLE_REAL_ADAPTERS=true`, explicit endpoints and an administrator request. A probe tests connectivity only; it does not issue remote commands.
- Future script-compatible agents and build toolchains must run as separately
  deployed, one-shot workers with no management database credentials. Script,
  shell and interpreter handlers remain evidence-blocked. Their messages must
  be versioned and covered by golden vectors before enabling the adapter.

## Consequences

The management lifecycle, persistence, UI, audit events and reconnect behavior can be tested now without coupling execution to the API process. This is a transitional compatibility deviation: actual framing, build and remote-session behavior moves to isolated workers only after its protocol evidence and runtime gates are complete.
