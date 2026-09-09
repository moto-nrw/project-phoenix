import { vi } from "vitest";

/**
 * Deterministic test clock (#3101).
 *
 * `src/test/setup-common.ts` freezes `Date` at {@link TEST_CLOCK_INSTANT} before
 * every test and restores the real clock afterwards. Only `Date` is mocked:
 * `setTimeout`, `setInterval`, promises and `waitFor` keep running on real
 * timers, so async tests behave exactly as before. The freeze reaches every
 * `new Date()` / `Date.now()` in the rendered component tree and in libraries
 * (react-day-picker's "today", date-fns, `berlinTodayISO()` …) — mocking a
 * domain hook such as `useBerlinToday` no longer leaves the calendar on the
 * host date.
 *
 * Change the time with {@link setTestClock}; it accepts an ISO instant or a
 * `Date`. Fake timers stay available: `vi.useFakeTimers()` starts at the
 * frozen instant, `vi.advanceTimersByTime()` moves it, and the setup hook
 * uninstalls them after the test. To hand the timers back to the real event
 * loop in the middle of a test, call {@link releaseFakeTimers} instead of
 * `vi.useRealTimers()` — the latter also drops the frozen clock, which the
 * oxlint rule `test-clock/no-real-clock` rejects outside `afterEach`/`afterAll`.
 *
 * Wednesday 9 September 2026, 12:00 Berlin (CEST, ISO week 37): a plain
 * weekday in the middle of a week, a month and a DST period. Tests covering
 * Berlin midnight, week changes or the DST switch pick their own instant.
 */
const TEST_CLOCK_ISO = "2026-09-09T12:00:00+02:00";

/** The frozen instant every test starts on. */
export const TEST_CLOCK_INSTANT = new Date(TEST_CLOCK_ISO);

/** The Berlin calendar day of {@link TEST_CLOCK_INSTANT}. */
export const TEST_CLOCK_TODAY = "2026-09-09";

/**
 * Move the frozen clock to `instant` (ISO string or `Date`). Works with and
 * without `vi.useFakeTimers()`; the setup hook restores the real clock after
 * the test.
 */
export function setTestClock(instant: string | Date): void {
  const date = instant instanceof Date ? instant : new Date(instant);
  if (Number.isNaN(date.getTime())) {
    throw new TypeError(`setTestClock: invalid instant ${String(instant)}`);
  }
  vi.setSystemTime(date);
}

/**
 * Freeze the clock at the shared default. The setup hook calls this before
 * every test; call it yourself only after {@link useRealClock}.
 */
export function freezeTestClock(): void {
  vi.setSystemTime(TEST_CLOCK_INSTANT);
}

/**
 * Replace fake timers with the real event loop while keeping the clock frozen
 * at its current fake time. Use this where a test used to call
 * `vi.useRealTimers()` after advancing fake timers.
 */
export function releaseFakeTimers(): void {
  const now = vi.getMockedSystemTime() ?? TEST_CLOCK_INSTANT;
  vi.useRealTimers();
  vi.setSystemTime(now);
}

/**
 * Hand the current test the real system clock. This is the narrow exception
 * for behaviour that must observe wall-clock time (e.g. measuring elapsed
 * time). The oxlint rule `test-clock/no-real-clock` requires the reason as a
 * non-empty string literal so every exception is reviewed in place:
 *
 *   useRealClock("measures real elapsed time between two fetches");
 *
 * The setup hook re-freezes the clock for the next test.
 */
export function useRealClock(reason: string): void {
  if (reason.trim() === "") {
    throw new TypeError("useRealClock: a reviewed reason is required");
  }
  vi.useRealTimers();
}

/** Whether `Date` currently reports the frozen test clock (or a fake timer clock). */
export function isTestClockFrozen(): boolean {
  return vi.getMockedSystemTime() !== null;
}
