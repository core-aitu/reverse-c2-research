import test from 'node:test';
import assert from 'node:assert/strict';
import { pluginCatalog } from '../src/researchPlugins/catalog.ts';
import { buildReports } from '../src/researchPlugins/reports.ts';
const ids = pluginCatalog.map(p => p.id);
const values = report => Object.fromEntries(report.rows.map(r => [r.label, r.value]));
test('catalog has three unique versioned reports', () => {
  assert.equal(new Set(ids).size, 3);
  assert.ok(pluginCatalog.every(p => /^\d+\.\d+\.\d+$/.test(p.version)));
});
test('baseline quality counts', () => assert.deepEqual(values(buildReports('baseline', ['quality']).reports[0]), {'Total observations':6, Accepted:6, Missing:0, Rejected:0, Duplicates:0}));
test('quality issues are separated', () => assert.deepEqual(values(buildReports('quality', ['quality']).reports[0]), {'Total observations':8, Accepted:5, Missing:1, Rejected:1, Duplicates:1}));
test('coverage uses unique observations', () => {
  const rows = values(buildReports('quality', ['coverage']).reports[0]);
  assert.equal(rows['Classroom A: acceptance'], 50);
  assert.equal(rows['Classroom B: acceptance'], 100);
});
test('temperature report excludes invalid observations', () => {
  const rows = values(buildReports('quality', ['temperature']).reports[0]);
  assert.equal(rows['Classroom A: average'], 21.5);
  assert.equal(rows['Classroom B: average'], 23);
});
test('selection is respected and catalog order is stable', () => {
  assert.deepEqual(buildReports('baseline', ['temperature','quality']).reports.map(p => p.id), ['quality','temperature']);
  assert.deepEqual(buildReports('baseline', []).reports, []);
});
test('unknown fixture, plugin and duplicate selections fail closed', () => {
  assert.throws(() => buildReports('external', ids), RangeError);
  assert.throws(() => buildReports('baseline', ['external']), RangeError);
  assert.throws(() => buildReports('baseline', ['quality','quality']), RangeError);
});
test('results are deterministic and isolated across calls', () => {
  const first = buildReports('quality', ids);
  assert.deepEqual(first, buildReports('quality', ids));
  first.reports[0].rows[0].value = -1;
  assert.equal(buildReports('quality', ids).reports[0].rows[0].value, 8);
});
