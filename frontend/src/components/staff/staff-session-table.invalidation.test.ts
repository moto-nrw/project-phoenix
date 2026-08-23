import { describe, expect, it } from "vitest";

import { isStaleAfterSessionSave } from "./staff-session-table";

// A correction or backfill in the admin edit modal rewrites the day's Ist —
// and with it the Gutschrift and the Saldo the server now projects per day
// (#2443). Every cache showing those numbers has to be invalidated, or the
// table keeps the pre-save Saldo while the Monatskarte above it has already
// updated. useSWRAuth prefixes keys with the tenant slug, so the predicate
// matches with includes.
describe("isStaleAfterSessionSave", () => {
  it("invalidates the sessions, absences, month and daily-projection caches", () => {
    for (const key of [
      "phoenix:staff-history-42-2026-08-01-2026-08-31",
      "phoenix:staff-absences-42-2026-08-01-2026-08-31",
      "phoenix:staff-month-summary-42-2026-8",
      "phoenix:staff-schedule-targets-42-2026-08-01-2026-08-31",
      // Own-service portal keys (no staff id): a manager correcting their OWN
      // days also has the self-service table and weekly KPI open, which key
      // without an id and read the same projection.
      "phoenix:time-tracking-month-summary-2026-8",
      "phoenix:time-tracking-schedule-targets-2026-08-01-2026-08-31",
    ]) {
      expect(isStaleAfterSessionSave(key, "42")).toBe(true);
    }
  });

  it("leaves the target-only account chart and unrelated caches alone", () => {
    for (const key of [
      // Fetched with target_only=true: pure Soll, which a session edit cannot
      // change. Refetching the whole account range on every correction would
      // be a large request for an unchanged answer.
      "phoenix:staff-schedule-targets-account-42-2026-01-01-2026-12-31",
      "phoenix:staff-schedule-42",
      "phoenix:time-tracking-holidays-2026-08-01-2026-08-31",
      "phoenix:time-tracking-closing-days-2026-08-01-2026-08-31",
      "time-tracking-config",
    ]) {
      expect(isStaleAfterSessionSave(key, "42")).toBe(false);
    }
  });

  it("scopes the daily projection to the edited staff member", () => {
    expect(
      isStaleAfterSessionSave(
        "phoenix:staff-schedule-targets-7-2026-08-01-2026-08-31",
        "42",
      ),
    ).toBe(false);
  });

  it("ignores non-string SWR keys", () => {
    expect(isStaleAfterSessionSave(null, "42")).toBe(false);
    expect(isStaleAfterSessionSave(["staff-history-42"], "42")).toBe(false);
    expect(isStaleAfterSessionSave(undefined, "42")).toBe(false);
  });
});
