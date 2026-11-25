# Research plugins (built-in report catalog)

Open **Research → Research Plugins**, choose a classroom measurement fixture,
select the reports, and click **Run selected reports**. Results are local to the
browser and reset when the recording or selection changes.

| Report | Output |
| --- | --- |
| Data quality | Total, accepted, missing, rejected and duplicate observations |
| Station coverage | Accepted/unique observations and acceptance percentage |
| Temperature summary | Mean and last accepted temperature per classroom |

Coverage excludes duplicate observation IDs. The denominator is all unique
observations for that station, including missing and rejected values. A missing
average is displayed as a dash, not zero. Reports use the existing measurement
replay model and do not change its fixtures or validation rules.

Implementation: `web/src/researchPlugins/catalog.ts` defines the closed catalog;
`reports.ts` calculates the three fixed reports; `ResearchPlugins.tsx` renders
selection and results. It is a compiled-in report catalog, not a general-purpose
plugin loader. There is no arbitrary code, external plugin installation,
operational event subscription, server extension, or network access. It does not
change the API, database or legacy compatibility boundary.

Run tests from `web/`: `pnpm run test:plugins` on the pinned Node version, or
`node --experimental-strip-types --test tests/researchPlugins.test.mjs` on Node
22.16. The workflow runs these tests alongside the other frontend checks.
