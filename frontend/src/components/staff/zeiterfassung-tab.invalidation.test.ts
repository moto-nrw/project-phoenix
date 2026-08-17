import { describe, expect, it } from "vitest";

import { berlinTodayISO } from "~/lib/date-helpers";
import { startOfWeek, toDateKey } from "~/lib/staff-metrics-helpers";
import {
  isStaleAfterMonthReopen,
  resolveInitialTimeTrackingDate,
} from "./zeiterfassung-tab";

describe("isStaleAfterMonthReopen", () => {
  it("invalidates staff, self-service, overview, dashboard, and close-status caches", () => {
    for (const key of [
      "phoenix:staff-month-summary-42-2026-7",
      "phoenix:time-tracking-month-summary-2026-7",
      "phoenix:staff-time-accounts-2026-7",
      "phoenix:staff-dashboard-summary-month",
      "phoenix:staff-month-close-2026-7",
    ]) {
      expect(isStaleAfterMonthReopen(key, "42")).toBe(true);
    }
  });

  it("leaves unrelated and other staff summary caches alone", () => {
    expect(
      isStaleAfterMonthReopen("phoenix:staff-month-summary-17-2026-7", "42"),
    ).toBe(false);
    expect(isStaleAfterMonthReopen("phoenix:staff-list", "42")).toBe(false);
    expect(isStaleAfterMonthReopen(null, "42")).toBe(false);
  });
});

describe("resolveInitialTimeTrackingDate", () => {
  it("initializes the view to the selected historic week", () => {
    const initialDate = resolveInitialTimeTrackingDate("2026-01-08");

    expect(toDateKey(startOfWeek(initialDate))).toBe("2026-01-05");
  });

  it("falls back to today for an invalid URL date", () => {
    expect(toDateKey(resolveInitialTimeTrackingDate("2026-02-31"))).toBe(
      berlinTodayISO(),
    );
  });
});
