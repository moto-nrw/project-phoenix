# Frontend test clock and timers

Read before writing or diagnosing a date- or time-sensitive Vitest test.
Paths start at `frontend/`; commands run from `frontend/`.

## Scope

Every Vitest test in both projects (`app-dom` and `api-node`) runs on a
deterministic clock (#3101). `src/test/setup-common.ts` freezes `Date` at
`TEST_CLOCK_INSTANT` before each test and restores the real clock afterwards.
`vitest.config.ts` pins the zone to `Europe/Berlin`, so the frozen instant is
always Wednesday, 9 September 2026, 12:00 Berlin time (`TEST_CLOCK_TODAY` =
`2026-09-09`).

The freeze covers `new Date()` and `Date.now()` everywhere in the worker: the
test body, the rendered component tree, and libraries such as react-day-picker
and date-fns. Mocking a domain hook (`useBerlinToday`, `todayISO`) no longer
leaves a calendar or a relative-time label on the CI host's date.

Only `Date` is frozen. `setTimeout`, `setInterval`, promises and Testing
Library's `waitFor` keep running on real timers; async tests behave as before.

## Use

Helpers live in `src/test/clock.ts` (`~/test/clock`).

| Need | Use | Never |
|---|---|---|
| The default instant or day | `TEST_CLOCK_INSTANT`, `TEST_CLOCK_TODAY` | a literal copied from the setup |
| Another point in time (Berlin midnight, week change, DST switch) | `setTestClock("2026-03-29T01:30:00+01:00")` in the test or `beforeEach` | `vi.useFakeTimers({ toFake: ["Date"] })` boilerplate; the setup already freezes `Date` |
| Control timers (`setInterval`, debounce) | `vi.useFakeTimers()`; it starts at the frozen instant, `vi.advanceTimersByTime` moves clock and timers | assuming fake timers start at the host time |
| Real event loop again mid-test | `releaseFakeTimers()`: timers become real, the clock stays where the fake timers left it | `vi.useRealTimers()` in a test body or `before*` hook |
| Cleanup | nothing; the setup restores the clock and uninstalls fake timers after every test. `afterEach(() => vi.useRealTimers())` stays allowed | `vi.useRealTimers()` inside `try … finally` |
| Wall-clock time on purpose | `useRealClock("reason")` with the reviewed reason as a string literal | `vi.getRealSystemTime()`, `vi.stubGlobal("Date", …)`, `globalThis.Date = …` |

A test that mocks a domain hook to one day while the component tree reads
another day is deterministic but asserts a mismatch. Prefer `setTestClock` and
let the hook read the frozen clock unless the mismatch is the behaviour under
test.

Fixtures built relative to "now" (`Date.now() + 60_000`, `new Date()`) stay
valid: fixture and product code see the same frozen instant. Fixtures that use
`Date.now()` for uniqueness collide within one test; use a counter instead.

## Enforcement

- `src/test/clock.test.tsx` is the regression test of the mechanism: a pinned
  hook plus the real kit `DatePicker` renders the today label on the frozen
  day, `setTestClock` covers Berlin midnight, a week change and the DST
  switch, and time and timer state reset between tests.
- oxlint rule `test-clock/no-real-clock`
  (`scripts/oxlint-plugin-test-clock.mjs`, tested in
  `scripts/oxlint-plugin-test-clock.test.ts`) runs in `pnpm run check` and CI.
  It rejects `vi.useRealTimers()` outside `afterEach`/`afterAll`,
  `useRealClock()` without a non-empty literal reason, `vi.getRealSystemTime()`
  and any replacement of the global `Date`. There is no baseline and no
  allowlist; the exception is the reason inside the `useRealClock` call.
- `.claude/rules/no-test-modifications.md` applies: a date-dependent failure is
  fixed by pinning the clock, not by loosening the assertion.
