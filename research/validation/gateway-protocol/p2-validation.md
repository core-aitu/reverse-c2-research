# Gateway and beacon P2 validation gate

This note records the operational UI and verification gate for the bounded
clean-room gateway. It does not add evidence about an unobserved legacy wire
session.

## Operational surface

The console renders enrolled beacons and simulator fixtures in separate
sections. The real-beacon view exposes protocol/build version, allowlisted
capabilities, platform, first/last/next check-in, online/degraded/offline
health, cursor/sequence, identity generation, leases, dispatch/retry counts and
structured task errors. Administrative disable, revoke and cancellation
actions require browser confirmation.

Enrollment secrets are returned only by the create response and held only in
ephemeral component state until hidden or navigation/reload. The catalog
endpoint returns token ID, timestamps and creator only. Real task management
responses deliberately omit task and result payloads. Browser end-to-end tests
also assert that one-time tokens and command output are absent from
console messages.

Audit history accepts bounded beacon and task correlation IDs. Values outside
`[A-Za-z0-9._:-]` or longer than 128 characters are rejected before reaching
SQL.

## Automated matrix

- codec golden vectors, round trips and malformed seed corpus;
- bounded fuzz target for registration, check-in, batch, result, ack and
  receipt decoders;
- v1/v2 task state-machine, backoff, handler timeout/output/crash/cancellation and
  repeated cleanup tests;
- direct exec plus explicit sh/cmd/PowerShell selection, bounded streaming
  output, replay/duplicate ordering and process-tree termination tests;
- S3 upload/download/Range, multipart retry/abort, legacy bytea migration and
  remote-file digest validation;
- signature, certificate, downgrade, replay and duplicate-result negatives;
- PostgreSQL concurrent lease/result test under the race detector;
- PostgreSQL + gateway + real beacon enrollment, exec, shell, output archive,
  cancellation and file-retrieval lifecycle;
- gateway outage/restart and beacon restart during active tasks without repeat
  execution;
- reproducible cross-platform release matrix plus fresh launch-bundle binary;
- Compose smoke plus rendered Chromium bundle/download, live output,
  cancellation, remote-file and audit workflows.

Run the short gate with the commands in `reimplementation/README.md`. Run the
continuous gate with `make soak SOAK_DURATION=4h`; the duration is injected
into the same real integration test and rotates v1 echo, v2 exec, v2 shell and
remote-file retrieval once per second while checking dispatch counts,
goroutines and file descriptors.

## Promotion rule

`gateway-framing` and `agent-safe-task-execution` remain `implemented`. They
may move to `runtime-verified` only after all short gates pass on the target
revision and a multi-hour soak completes on that same revision with no race,
goroutine, process, file or socket leak evidence. An available soak command is
not evidence that the soak completed.

## Local validation record

On 2026-11-27 the current workspace passed the Go unit suite, race suite,
1,127,099 fuzz executions, Python contract suite, TypeScript production build,
20 browser-local unit tests, PostgreSQL concurrency test, PostgreSQL + gateway
+ beacon restart integration, live-stack smoke test, rendered Chromium E2E,
reproducible build comparison, SPDX/checksum generation and a fresh-binary TLS
plus signed-request echo lifecycle. The soak harness also completed its minimum one-minute
validation run. A multi-hour soak was not run, so this record does not satisfy
the promotion rule above.
