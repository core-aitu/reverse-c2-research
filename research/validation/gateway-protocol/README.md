# Gateway protocol evidence and validation

## What is observed

The pinned managed beacon is `d5e7e80c…7569d` (MVID
`e10959e7-1725-4dc7-9ff7-d7dd25e9f0e8`; REA parent Evidence
`ev_d79c955f675f326656ad1eb9c266be86912be35e13b6e623314f1ed0fbe64c45`).
Static source/PDB and IL evidence agree on these HTTP operations:

- `POST /threads/{sessionId}/register` with a compressed `RegistrationRequest`;
- `GET /threads/{sessionId}/messages?filter={milliseconds}` for check-in;
- `POST /threads/{sessionId}/messages` for `RunScriptResponse`;
- `POST /threads/{sessionId}/notes` for typed errors.

The legacy body has no inner magic, version, type tag, or length field. HTTP
selects the DTO and provides the outer length. Body magic is gzip `1f 8b`.
The uncompressed serialization is `.NET BinaryWriter`: little-endian integers,
one-byte booleans, UTF-8 strings prefixed by a 7-bit encoded byte length,
`DateTime.ToBinary()` and five signed `Int32` TimeSpan components. Task arrays
begin with a signed `Int32` count. The optional response-authentication mechanism
is an `X-Specter-Signature` RSA/SHA-256 signature; it is server authentication,
not beacon enrollment.

## What remains unobserved

The controlled Linux/.NET 10 replay opened loopback TCP/TLS connections but the
legacy `HttpWebRequest` closed them before sending HTTP bytes. Therefore the
repository still does **not** claim a real legacy registration/check-in capture.
The exact failure boundary is recorded in `legacy-replay-summary.json`. A
compatible Windows/.NET Framework or Mono laboratory is still needed for that
checkbox and for authoritative malformed-frame rejection behavior of the
original server.

## Implemented clean-room gateway

The reimplementation deliberately keeps remote script DTOs out. It reuses the
observed gzip/BinaryWriter primitives and adds explicit version/authentication
headers, versioned task envelopes, sequence/cursor/nonce/timestamp replay
protection, one-use enrollment, overlapping identity rotation/revocation,
atomic task leasing and idempotent result submission. The request HMAC is a
clean-room safety mechanism; it is not mislabelled as the legacy product's
server-side RSA response signature. `golden-vectors.json` contains fictional,
secret-free vectors for registration, empty check-in, a two-task batch and a
successful result. Envelope v1 retains `echo`, `system-info` and `sleep`;
envelope v2 provides direct `exec`, explicit `sh`/`cmd`/PowerShell selection,
bounded streamed output and digest-verified remote-file retrieval backed by
private S3 storage.
`malformed-vectors.json` records the safe gateway rejection corpus; it is not
mislabelled as observed legacy-server behavior.

The current integration suite records restart behavior during active work,
requires `dispatchCount=1`, archives output, terminates cancelled process trees
and validates downloaded file bytes. The capability is `implemented`, not
`runtime-verified` legacy compatibility until the four-hour promotion gate is
recorded for the same revision.

P1 state, resilience, transport, worker, transfer and build validation is
listed in `p1-validation.md`.

Run codec and beacon checks with:

```bash
cd reimplementation
go test ./internal/gateway ./internal/beacon
```
