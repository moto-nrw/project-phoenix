import { describe, expect, it } from "vitest";

import type {
  BackendCalendarPeriod,
  CalendarPeriod,
} from "./calendar-period-helpers";
import {
  findPeriodForDate,
  mapPeriodsForDates,
  mapPeriodWarnings,
  mapPeriodWithWarnings,
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

  describe("mapPeriodWarnings / mapPeriodWithWarnings", () => {
    const backendPeriod: BackendCalendarPeriod = {
      id: 5,
      tenant_id: 1,
      name: "Schuljahr 2026/2027",
      period_type: "school_year",
      start_date: "2026-08-01",
      end_date: "2027-07-31",
      week_cycle_length: 1,
      week_cycle_anchor: null,
      is_active: true,
      created_at: "2026-05-01T00:00:00Z",
      updated_at: "2026-05-01T00:00:00Z",
    };

    it("maps backend warnings to camelCase with string IDs", () => {
      expect(
        mapPeriodWarnings([
          {
            code: "overlapping_active_periods",
            message: "Zeitraum überschneidet sich mit aktiven Zeiträumen",
            overlapping_period_ids: [3, 4],
            overlapping_period_names: ["Halbjahr 1", "Ferien"],
          },
        ]),
      ).toEqual([
        {
          code: "overlapping_active_periods",
          message: "Zeitraum überschneidet sich mit aktiven Zeiträumen",
          overlappingPeriodIds: ["3", "4"],
          overlappingPeriodNames: ["Halbjahr 1", "Ferien"],
        },
      ]);
    });

    it("defaults missing warning arrays to empty", () => {
      expect(mapPeriodWarnings(undefined)).toEqual([]);
      expect(mapPeriodWarnings(null)).toEqual([]);
      expect(
        mapPeriodWarnings([
          { code: "overlapping_active_periods", message: "Überschneidung" },
        ]),
      ).toEqual([
        {
          code: "overlapping_active_periods",
          message: "Überschneidung",
          overlappingPeriodIds: [],
          overlappingPeriodNames: [],
        },
      ]);
    });

    it("splits a backend period into { period, warnings }", () => {
      const result = mapPeriodWithWarnings({
        ...backendPeriod,
        warnings: [
          {
            code: "overlapping_active_periods",
            message: "Überschneidung",
            overlapping_period_ids: [3],
            overlapping_period_names: ["Halbjahr 1"],
          },
        ],
      });

      expect(result.period).toMatchObject({
        id: "5",
        name: "Schuljahr 2026/2027",
      });
      expect(result.warnings).toEqual([
        {
          code: "overlapping_active_periods",
          message: "Überschneidung",
          overlappingPeriodIds: ["3"],
          overlappingPeriodNames: ["Halbjahr 1"],
        },
      ]);
    });

    it("returns empty warnings when the backend omits the field", () => {
      const result = mapPeriodWithWarnings(backendPeriod);
      expect(result.warnings).toEqual([]);
    });
  });
});
