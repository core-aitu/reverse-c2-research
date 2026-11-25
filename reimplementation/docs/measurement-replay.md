# Offline measurement replay

Open **Research → Measurement Replay** after signing in. Choose a fixed fictional
classroom temperature recording, press Play, or seek directly to an observation.
Playback uses display time rather than the recorded time difference. The complete
recording and its original arrival order remain unchanged.

The pure TypeScript reducer (`web/src/measurementReplay.ts`) recomputes statistics
from the displayed prefix, preventing future readings from leaking into earlier
frames when seeking backward. Duplicate IDs count once for statistics; missing
values and values outside the fictional instrument's accepted range (-40 to 60 °C)
are counted separately. The mean and last accepted value exclude all three kinds
of anomalies. Duplicate rows and invalid readings remain visible in the timeline.

This standalone browser module uses only two built-in fictional recordings. It
does not contact the backend, operate devices, modify session state, accept live
feeds, persist data, or implement a Beacon or an agent protocol. It is an extension
of the research UI, not a recovered original component.

## Tests

From `web/`, run `pnpm run test:replay` on the pinned Node version. For Node 22.16,
run `node --experimental-strip-types --test tests/measurementReplay.test.mjs`.
The tests cover empty frames, means, anomalies, backward seeks, last valid value,
invalid seek positions, and independent fixtures. Browser interaction requires a
separate full frontend build and browser check.
