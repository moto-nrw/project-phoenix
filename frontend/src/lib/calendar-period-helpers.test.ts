import { describe, expect, it } from "vitest";

import type { CalendarPeriod } from "./calendar-period-helpers";
import {
  findPeriodForDate,
  mapPeriodsForDates,
  uniqueAssignedPeriods,
} from "./calendar-period-helpers";

function period(
  id: string,
  name: string,
  startDate: string,
  endDate: string,
  isActive = true,
): CalendarPeriod {
  return {
    id,
    tenantId: "1",
    name,
    periodType: "school_year",
    startDate,
    endDate,
    weekCycleLength: 1,
    weekCycleAnchor: null,
    isActive,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
}

describe("calendar-period-helpers", () => {
  describe("findPeriodForDate", () => {
    it("returns the active period covering the date", () => {
      const periods = [
        period("1", "Schuljahr 2025/2026", "2025-08-01", "2026-07-31"),
      ];

      expect(findPeriodForDate(periods, "2026-04-28")?.id).toBe("1");
    });

    it("ignores inactive periods", () => {
      const periods = [
        period("1", "Inactive", "2025-08-01", "2026-07-31", false),
      ];

      expect(findPeriodForDate(periods, "2026-04-28")).toBeNull();
    });

    it("uses lowest id when active periods overlap", () => {
      const periods = [
        period("9", "Projektwoche", "2026-04-27", "2026-05-01"),
        period("2", "Schuljahr", "2025-08-01", "2026-07-31"),
      ];

      expect(findPeriodForDate(periods, "2026-04-28")?.id).toBe("2");
    });
  });

  describe("mapPeriodsForDates", () => {
    it("maps a boundary week day by day", () => {
      const periods = [
        period("1", "Osterferien", "2026-04-06", "2026-04-21"),
        period("2", "Schuljahr", "2026-04-22", "2026-07-31"),
      ];

      const assignments = mapPeriodsForDates(periods, [
        "2026-04-20",
        "2026-04-21",
        "2026-04-22",
      ]);

      expect(assignments.map((a) => a.period?.name ?? null)).toEqual([
        "Osterferien",
        "Osterferien",
        "Schuljahr",
      ]);
    });

    it("keeps missing days explicit", () => {
      const assignments = mapPeriodsForDates([], ["2026-04-20"]);

      expect(assignments).toEqual([{ date: "2026-04-20", period: null }]);
    });
  });

  describe("uniqueAssignedPeriods", () => {
    it("returns each assigned period once in id order", () => {
      const holiday = period("7", "Ferien", "2026-04-01", "2026-04-14");
      const school = period("2", "Schuljahr", "2026-04-15", "2026-07-31");

      const unique = uniqueAssignedPeriods([
        { date: "2026-04-13", period: holiday },
        { date: "2026-04-14", period: holiday },
        { date: "2026-04-15", period: school },
        { date: "2026-04-16", period: null },
      ]);

      expect(unique.map((p) => p.id)).toEqual(["2", "7"]);
    });
  });
});
