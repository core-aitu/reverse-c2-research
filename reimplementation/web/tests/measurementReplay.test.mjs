import test from 'node:test';
import assert from 'node:assert/strict';
import { recordings, replay } from '../src/measurementReplay.ts';

test('initial frame contains no future data', () => {
  assert.deepEqual(replay(recordings()[0], 0), { position: 0, events: [], stations: [], duplicates: 0, valid: 0, missing: 0, rejected: 0 });
});
test('baseline averages and last accepted readings', () => {
  const result = replay(recordings()[0], 6);
  assert.equal(result.valid, 6);
  assert.equal(result.stations[0].average, 22);
  assert.equal(result.stations[1].average, 67 / 3);
  assert.equal(result.stations[0].last, 23);
  assert.equal(result.duplicates + result.missing + result.rejected, 0);
});
test('bad data stays visible without corrupting averages', () => {
  const result = replay(recordings()[1], 8);
  assert.equal(result.events.length, 8);
  assert.equal(result.valid, 5);
  assert.equal(result.missing, 1);
  assert.equal(result.rejected, 1);
  assert.equal(result.duplicates, 1);
  assert.equal(result.stations[0].average, 21.5);
  assert.equal(result.stations[1].average, 23);
});
test('seeking backwards drops later data', () => {
  const recording = recordings()[1];
  const before = replay(recording, 2);
  replay(recording, 8);
  assert.deepEqual(replay(recording, 2), before);
  assert.equal(before.duplicates + before.missing + before.rejected, 0);
});
test('missing and rejected readings do not overwrite last accepted value', () => {
  const recording = recordings()[1];
  assert.equal(replay(recording, 6).stations[0].last, 21);
});
test('invalid positions are rejected', () => {
  for (const position of [-1, 7, 1.5, NaN, Infinity]) assert.throws(() => replay(recordings()[0], position), RangeError);
});
test('fixture construction is independent of other callers', () => {
  const catalog = recordings();
  catalog[0].events[0].temperature = 50;
  assert.equal(recordings()[0].events[0].temperature, 21);
});
