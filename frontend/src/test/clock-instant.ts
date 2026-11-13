/**
 * The frozen test instant (#3101), without a Vitest import so the Playwright
 * kit pixel comparison (#6562) runs on the same clock. Tests import it from
 * `~/test/clock`; see the rationale there.
 */
const TEST_CLOCK_ISO = "2026-09-09T12:00:00+02:00";

/** The frozen instant every test starts on. */
export const TEST_CLOCK_INSTANT = new Date(TEST_CLOCK_ISO);

/** The Berlin calendar day of {@link TEST_CLOCK_INSTANT}. */
export const TEST_CLOCK_TODAY = "2026-09-09";
