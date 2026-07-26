import { describe, expect, it } from "vitest";

import { isStaleAfterModelSave } from "./arbeitszeitmodell-tab";

// After a work-time model save the contractual Soll changes, so the sibling
// Zeiterfassung / Übersicht tabs' cached targets and month aggregates go stale
// and must be invalidated — otherwise the open staff page shows the old Soll
// until a reload (#1842). useSWRAuth prefixes keys with the tenant slug, so the
// predicate matches with includes.
describe("isStaleAfterModelSave", () => {
  it("invalidates the date-valid targets and month aggregate caches", () => {
    for (const key of [
      "phoenix:staff-schedule-targets-42-2026-06-01-2026-06-30",
      "phoenix:staff-schedule-targets-account-42-2026-01-01-2026-07-16",
      "phoenix:staff-month-summary-42-2026-7",
      "phoenix:time-tracking-month-summary-2026-7",
      // Own-service portal keys (no staff id): a manager editing their OWN
      // model must also refresh the self-service daily table and weekly KPI,
      // which key without an id (#1842).
      "phoenix:time-tracking-schedule-targets-2026-06-01-2026-06-30",
    ]) {
      expect(isStaleAfterModelSave(key)).toBe(true);
    }
  });

  it("leaves unrelated caches alone", () => {
    for (const key of [
      "phoenix:staff-schedule-42",
      "phoenix:staff-history-42-2026-06-01-2026-06-30",
      "phoenix:staff-absences-42-2026-06-01-2026-06-30",
      "time-tracking-config",
    ]) {
      expect(isStaleAfterModelSave(key)).toBe(false);
    }
  });

  it("ignores non-string SWR keys", () => {
    expect(isStaleAfterModelSave(null)).toBe(false);
    expect(isStaleAfterModelSave(["staff-month-summary-42"])).toBe(false);
    expect(isStaleAfterModelSave(undefined)).toBe(false);
  });
});
