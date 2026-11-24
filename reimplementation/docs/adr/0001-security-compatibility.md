# ADR 0001: secure contract compatibility

## Decision

The reimplementation preserves observed REST paths, JSON shapes and event names, but does not reproduce unsafe authorization behavior.

- Administrative REST operations require a valid JWT and an explicit role.
- WebSocket connections require a short-lived, single-use ticket and enforce Origin.
- Refresh tokens are stored as SHA-256 digests, rotated on use and revoked on logout.
- Filesystem-facing adapters must prove that resolved paths remain under their configured root.
- AI functions, payload pipelines and agent tasks cannot be invoked anonymously.

These differences are intentional compatibility breaks associated with FND-036 through FND-041 and are covered by negative tests as each subsystem is implemented.
