# ADR 0001: Safe compatibility boundary

- Status: accepted
- Date: 2026-11-25
- Scope: SpecterInsight 7.0.0 clean-room management-plane contract

## Decision

All 202 method/path pairs in the independently described REST manifest are registered and authenticated according to the new security policy. Data-management operations use typed PostgreSQL tables where an independently verified schema is available and the persistent `app_resources` registry for bounded compatibility metadata elsewhere.

The reimplementation does not reproduce capabilities whose primary effect is remote command execution, credential use, covert payload creation/loading, phishing delivery, or remote file transfer. Those routes fail closed with HTTP `501`, the stable code `CAPABILITY_NOT_IMPLEMENTED`, a safe alternative, and a `CapabilityBlocked` audit event.

The affected capability classes are:

- agent script execution and implant download;
- PowerShell, SSH, and WinRM command execution;
- SSH/WinRM upload and download;
- WinRM PowerShell, DLL, assembly, AMSI, and ETW loading;
- loader bundle or assembly materialization;
- phishing delivery;
- certificate private-key import;
- remote cached-file retrieval;
- state-changing pipeline execution over `GET`.

Listener, tunnel, remote-session, Tor, ACME, SMTP, WebDAV, AI-provider, and update operations are fixture/configuration semantics unless a real adapter is explicitly enabled. Fixture operations do not bind a socket, start a process, contact an external host, issue a certificate, send mail, or install an update. Legacy `password`, `secret`, private-key, and access/refresh-token fields are redacted before compatibility metadata is persisted.

## Consequences

- Method, path, authentication, JSON handling, persistence, status/error handling, and audit behavior can be tested without creating an operational C2 or delivery system.
- The internal `SIR1` protocol remains a simulator contract. It is not claimed to be the unverified legacy beacon protocol.
- Safe payload pipelines are deterministic data DAGs in one-shot workers. They are not shell or compiler pipelines.
- A future expansion requires a new ADR, an isolated execution design, explicit operator configuration, and tests proving that management credentials and the management database are unavailable to workers.

## Verification

`TestManifestRoutesAreRegistered` loads `contracts/manifest.json`, asserts the pinned count of 202 unique operations, sends each method/path through the real router, and fails on an unregistered route. Integration smoke tests verify representative persistent fixture workflows and the `501` boundaries above.
