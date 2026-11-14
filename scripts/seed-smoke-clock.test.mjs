import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { dirname, join } from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

import { berlinInstant, easterSunday, isCareDay, nextCareDayStart, nrwHolidays } from './seed-smoke-clock.mjs';

const script = join(dirname(fileURLToPath(import.meta.url)), 'seed-smoke-clock.mjs');
// Fixed instants: Berlin is UTC+1 in winter and UTC+2 in summer.
const at = iso => Date.parse(iso);

test('a weekday morning before 07:20 starts the same day', () => {
  assert.equal(nextCareDayStart(at('2026-11-12T05:00:00Z')), at('2026-11-12T06:20:00Z'));
  assert.equal(nextCareDayStart(at('2026-11-13T05:00:00Z')), at('2026-11-13T06:20:00Z'));
});

test('after 07:20 on Monday to Thursday the next day starts', () => {
  assert.equal(nextCareDayStart(at('2026-11-16T06:20:00Z')), at('2026-11-17T06:20:00Z'));
  assert.equal(nextCareDayStart(at('2026-11-18T12:00:00Z')), at('2026-11-19T06:20:00Z'));
});

test('Friday after 07:20, Saturday and Sunday move on to Monday', () => {
  const monday = at('2026-11-16T06:20:00Z');
  assert.equal(nextCareDayStart(at('2026-11-13T06:21:00Z')), monday);
  assert.equal(nextCareDayStart(at('2026-11-13T22:59:59Z')), monday);
  assert.equal(nextCareDayStart(at('2026-11-14T10:00:00Z')), monday);
  assert.equal(nextCareDayStart(at('2026-11-15T22:00:00Z')), monday);
});

test('Nordrhein-Westfalen holidays are skipped', () => {
  // Thursday before Christmas: 25 and 26 December, then the weekend.
  assert.equal(nextCareDayStart(at('2026-12-24T08:00:00Z')), at('2026-12-28T06:20:00Z'));
  // Maundy Thursday: Karfreitag, Easter weekend with the clock change,
  // Ostermontag; Tuesday starts in summer time.
  assert.equal(nextCareDayStart(at('2027-03-25T08:00:00Z')), at('2027-03-30T05:20:00Z'));
  // Christi Himmelfahrt is a Thursday; the Friday after it is a care day.
  assert.equal(nextCareDayStart(at('2027-05-05T08:00:00Z')), at('2027-05-07T05:20:00Z'));
  // Allerheiligen on a Monday.
  assert.equal(nextCareDayStart(at('2027-10-29T08:00:00Z')), at('2027-11-02T06:20:00Z'));
});

test('the holidays match the feiertage NordrheinWestfalen calendar', () => {
  assert.equal(easterSunday(2026), '2026-04-05');
  assert.equal(easterSunday(2027), '2027-03-28');
  assert.deepEqual([...nrwHolidays(2026)].sort(), [
    '2026-01-01', '2026-04-03', '2026-04-06', '2026-05-01', '2026-05-14', '2026-05-25',
    '2026-06-04', '2026-10-03', '2026-11-01', '2026-12-25', '2026-12-26',
  ]);
  assert.equal(isCareDay('2026-11-16'), true);
  assert.equal(isCareDay('2026-11-14'), false);
  assert.equal(isCareDay('2027-01-01'), false);
});

test('07:20 Berlin follows the clock change', () => {
  assert.equal(berlinInstant('2026-10-26', { hour: 7, minute: 20 }), at('2026-10-26T06:20:00Z'));
  assert.equal(berlinInstant('2026-10-23', { hour: 7, minute: 20 }), at('2026-10-23T05:20:00Z'));
});

test('the command line prints Unix seconds', () => {
  const friday = String(at('2026-11-13T10:00:00Z') / 1000);
  assert.equal(execFileSync(process.execPath, [script, friday], { encoding: 'utf8' }).trim(), String(at('2026-11-16T06:20:00Z') / 1000));
  assert.throws(() => execFileSync(process.execPath, [script, 'friday'], { stdio: 'pipe' }));
});
