#!/usr/bin/env node
// Prints the Unix seconds of the next 07:20 Europe/Berlin on a care day, the
// time the seed smoke in .github/workflows/test.yml moves its runner clock to.
//
//   node scripts/seed-smoke-clock.mjs [unix-seconds]   (default: now)
//
// A care day is a weekday that is no public holiday of Nordrhein-Westfalen,
// the default federal state (operations.federal_state) of every seeded
// school. On a weekend the simulation skips the care day, and on a holiday
// the shift series and timetable plan nothing: the job then checked a day
// with no shifts and no staff at work (#6962). The holidays are those of
// github.com/wlbr/feiertage's NordrheinWestfalen, which the backend's school
// calendar uses (modules/schoolcalendar/internal/application/holidays.go).

const BERLIN = 'Europe/Berlin';
const START = { hour: 7, minute: 20 };
const DAY_MS = 24 * 60 * 60 * 1000;

const berlinFormat = new Intl.DateTimeFormat('en-CA', {
  timeZone: BERLIN,
  year: 'numeric', month: '2-digit', day: '2-digit',
  hour: '2-digit', minute: '2-digit', second: '2-digit',
  hourCycle: 'h23',
});

function berlinWallClock(ms) {
  const parts = Object.fromEntries(berlinFormat.formatToParts(new Date(ms)).map(part => [part.type, part.value]));
  return {
    date: `${parts.year}-${parts.month}-${parts.day}`,
    asUTC: Date.UTC(Number(parts.year), Number(parts.month) - 1, Number(parts.day),
      Number(parts.hour), Number(parts.minute), Number(parts.second)),
  };
}

function utcMidnight(date) {
  const [year, month, day] = date.split('-').map(Number);
  return Date.UTC(year, month - 1, day);
}

function isoDate(ms) {
  return new Date(ms).toISOString().slice(0, 10);
}

export function addDays(date, days) {
  return isoDate(utcMidnight(date) + days * DAY_MS);
}

// The instant of a Berlin wall-clock time on a date. 07:20 never falls into
// the hour a clock change skips or repeats, so the offset is unambiguous.
export function berlinInstant(date, { hour, minute }) {
  const wall = utcMidnight(date) + (hour * 60 + minute) * 60 * 1000;
  let instant = wall;
  for (let i = 0; i < 2; i++) {
    instant = wall - (berlinWallClock(instant).asUTC - instant);
  }
  return instant;
}

// Easter Sunday of the Gregorian calendar (Meeus/Jones/Butcher).
export function easterSunday(year) {
  const a = year % 19;
  const b = Math.floor(year / 100);
  const c = year % 100;
  const d = Math.floor(b / 4);
  const e = b % 4;
  const f = Math.floor((b + 8) / 25);
  const g = Math.floor((b - f + 1) / 3);
  const h = (19 * a + b - d - g + 15) % 30;
  const i = Math.floor(c / 4);
  const k = c % 4;
  const l = (32 + 2 * e + 2 * i - h - k) % 7;
  const m = Math.floor((a + 11 * h + 22 * l) / 451);
  const month = Math.floor((h + l - 7 * m + 114) / 31);
  const day = ((h + l - 7 * m + 114) % 31) + 1;
  return isoDate(Date.UTC(year, month - 1, day));
}

export function nrwHolidays(year) {
  const easter = easterSunday(year);
  const fixed = ['01-01', '05-01', '10-03', '11-01', '12-25', '12-26'].map(day => `${year}-${day}`);
  // Karfreitag, Ostermontag, Christi Himmelfahrt, Pfingstmontag, Fronleichnam.
  const movable = [-2, 1, 39, 50, 60].map(offset => addDays(easter, offset));
  return new Set([...fixed, ...movable]);
}

export function isCareDay(date) {
  const weekday = new Date(utcMidnight(date)).getUTCDay();
  if (weekday === 0 || weekday === 6) return false;
  return !nrwHolidays(Number(date.slice(0, 4))).has(date);
}

// The first 07:20 Berlin after nowMs that starts a care day. Counting in
// calendar days keeps the 23- and 25-hour days of a clock change exact.
export function nextCareDayStart(nowMs) {
  let date = berlinWallClock(nowMs).date;
  for (;;) {
    const start = berlinInstant(date, START);
    if (start > nowMs && isCareDay(date)) return start;
    date = addDays(date, 1);
  }
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const arg = process.argv[2];
  if (arg !== undefined && !/^\d+$/.test(arg)) {
    console.error('Usage: node scripts/seed-smoke-clock.mjs [unix-seconds]');
    process.exit(2);
  }
  const nowMs = arg === undefined ? Date.now() : Number(arg) * 1000;
  console.log(Math.floor(nextCareDayStart(nowMs) / 1000));
}
