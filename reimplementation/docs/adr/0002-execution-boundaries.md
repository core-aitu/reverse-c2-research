# ADR 0002: Execution boundaries for agents and payload pipelines

## Status

Accepted for the management-plane delivery.

## Context

The observed product contract includes agent tasking, remote-session commands, loaders and payload build pipelines. Reproducing those operations inside the web/API process would couple the management database to arbitrary code execution and would copy unsafe behavior that the clean-room plan explicitly excludes.

## Decision

- The management service implements a persistent agent protocol simulator: registration, check-in, bounded task dispatch and result submission.
- Simulator tasks are allowlisted to `echo` and `inventory`. They never spawn a process or connect to a remote host.
- Payload orchestration is represented as a versioned DAG. The management worker accepts only `input`, `literal`, `concat`, `base64`, `base64decode`, `sha256`, `uppercase` and `lowercase` nodes.
- Inputs, outputs, node counts and execution time are bounded. Shell, PowerShell, loader, filesystem and network nodes are rejected during validation.
- External adapters start in deterministic fixture mode. Real probes require `SPECTER_ENABLE_REAL_ADAPTERS=true`, explicit endpoints and an administrator request. A probe tests connectivity only; it does not issue remote commands.
- Future compatible agents and build toolchains must run as separately deployed, one-shot workers with no management database credentials. Their messages must be versioned and covered by golden vectors before enabling the adapter.

## Consequences

The management lifecycle, persistence, UI, audit events and reconnect behavior can be tested now without shipping covert execution capabilities. This is an intentional compatibility deviation: actual beacon framing, payload generation and remote command execution remain outside the management process until isolated workers and protocol evidence are reviewed.
