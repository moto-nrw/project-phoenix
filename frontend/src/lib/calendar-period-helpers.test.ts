import { describe, expect, it } from "vitest";

import type {
  BackendCalendarPeriod,
  CalendarPeriod,
} from "./calendar-period-helpers";
import {
  findPeriodForDate,
  weekCycleLabel,
  weekCycleSlotLetter,
  formatPeriodUsage,
  mapPeriodsForDates,
  mapPeriodWarnings,
  mapPeriodWithWarnings,
  uniqueAssignedPeriods,
  weekCycleSlotForDate,
  weekPatternForDate,
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
  describe("formatPeriodUsage", () => {
    it("formats singular and plural counts", () => {
      expect(formatPeriodUsage(1, 1)).toBe("1 Anmeldephase · 1 Regeltermin");
      expect(formatPeriodUsage(2, 3)).toBe("2 Anmeldephasen · 3 Regeltermine");
    });

    it("omits zero counts and supports a custom separator", () => {
      expect(formatPeriodUsage(1, 0)).toBe("1 Anmeldephase");
      expect(formatPeriodUsage(0, 2)).toBe("2 Regeltermine");
      expect(formatPeriodUsage(0, 0)).toBe("");
      expect(formatPeriodUsage(1, 2, " und ")).toBe(
        "1 Anmeldephase und 2 Regeltermine",
      );
    });

    it("formats every calendar-period reference category", () => {
      expect(
        formatPeriodUsage(0, 0, " · ", {
          activityGroupCount: 1,
          studentEnrollmentCount: 1,
          supervisorCount: 2,
          activityInstanceCount: 3,
        }),
      ).toBe(
        "1 Aktivitätsvorlage · 1 Schülerzuordnung · 2 Mitarbeitenden-Zuordnungen · 3 Termininstanzen",
      );
    });
  });

  describe("weekPatternForDate", () => {
    const cyclePeriod: CalendarPeriod = {
      ...period("9", "A/B-Halbjahr", "2026-05-01", "2026-12-31"),
      weekCycleLength: 2,
      weekCycleAnchor: "2026-05-04",
    };

    it("resolves the anchor week as A and alternates weekly", () => {
      expect(weekPatternForDate(cyclePeriod, "2026-05-04")).toBe(1);
      // Friday still belongs to the anchor week.
      expect(weekPatternForDate(cyclePeriod, "2026-05-08")).toBe(1);
      expect(weekPatternForDate(cyclePeriod, "2026-05-11")).toBe(2);
      expect(weekPatternForDate(cyclePeriod, "2026-05-18")).toBe(1);
    });

    it("wraps dates before the anchor", () => {
      expect(weekPatternForDate(cyclePeriod, "2026-04-27")).toBe(2);
      expect(weekPatternForDate(cyclePeriod, "2026-04-30")).toBe(2);
      expect(weekPatternForDate(cyclePeriod, "2026-04-20")).toBe(1);
    });

    it("returns null without a usable cycle", () => {
      expect(weekPatternForDate(null, "2026-05-04")).toBeNull();
      expect(weekPatternForDate(undefined, "2026-05-04")).toBeNull();
      expect(
        weekPatternForDate(
          period("1", "Ohne Zyklus", "2026-05-01", "2026-12-31"),
          "2026-05-04",
        ),
      ).toBeNull();
      expect(
        weekPatternForDate(
          { ...cyclePeriod, weekCycleAnchor: null },
          "2026-05-04",
        ),
      ).toBeNull();
    });

    it("returns null for cycle slots beyond B", () => {
      const threeWeekCycle = { ...cyclePeriod, weekCycleLength: 3 };
      expect(weekPatternForDate(threeWeekCycle, "2026-05-04")).toBe(1);
      expect(weekPatternForDate(threeWeekCycle, "2026-05-11")).toBe(2);
      expect(weekPatternForDate(threeWeekCycle, "2026-05-18")).toBeNull();
    });
  });

  describe("weekCycleSlotForDate", () => {
    const threeWeekCycle: CalendarPeriod = {
      ...period("9", "Drei-Wochen-Zyklus", "2026-05-01", "2026-12-31"),
      weekCycleLength: 3,
      weekCycleAnchor: "2026-05-04",
    };

    it("returns the raw slot including weeks beyond B", () => {
      expect(weekCycleSlotForDate(threeWeekCycle, "2026-05-04")).toBe(1);
      expect(weekCycleSlotForDate(threeWeekCycle, "2026-05-11")).toBe(2);
      expect(weekCycleSlotForDate(threeWeekCycle, "2026-05-18")).toBe(3);
      expect(weekCycleSlotForDate(threeWeekCycle, "2026-05-25")).toBe(1);
    });

    it("wraps dates before the anchor", () => {
      expect(weekCycleSlotForDate(threeWeekCycle, "2026-04-27")).toBe(3);
    });

    it("returns null without a usable cycle or with an invalid date", () => {
      expect(weekCycleSlotForDate(null, "2026-05-04")).toBeNull();
      expect(weekCycleSlotForDate(undefined, "2026-05-04")).toBeNull();
      expect(
        weekCycleSlotForDate(
          period("1", "Ohne Zyklus", "2026-05-01", "2026-12-31"),
          "2026-05-04",
        ),
      ).toBeNull();
      expect(
        weekCycleSlotForDate(
          { ...threeWeekCycle, weekCycleAnchor: null },
          "2026-05-04",
        ),
      ).toBeNull();
      expect(weekCycleSlotForDate(threeWeekCycle, "ungueltig")).toBeNull();
    });

    it("returns null for calendar-invalid dates instead of rolling them over", () => {
      // Date.UTC would normalize these to 2026-03-02 / 2027-01-01.
      expect(weekCycleSlotForDate(threeWeekCycle, "2026-02-30")).toBeNull();
      expect(weekCycleSlotForDate(threeWeekCycle, "2026-13-01")).toBeNull();
      expect(
        weekCycleSlotForDate(
          { ...threeWeekCycle, weekCycleAnchor: "2026-02-30" },
          "2026-05-04",
        ),
      ).toBeNull();
    });
  });

  describe("weekCycleSlotLetter", () => {
    it("maps 1-based slots to letters and falls back to numbers", () => {
      expect(weekCycleSlotLetter(1)).toBe("A");
      expect(weekCycleSlotLetter(2)).toBe("B");
      expect(weekCycleSlotLetter(3)).toBe("C");
      expect(weekCycleSlotLetter(4)).toBe("D");
      expect(weekCycleSlotLetter(26)).toBe("Z");
      expect(weekCycleSlotLetter(27)).toBe("27");
    });
  });

  describe("weekCycleLabel", () => {
    it("labels the actual cycle length", () => {
      expect(weekCycleLabel(2)).toBe("A/B-Wochen");
      expect(weekCycleLabel(3)).toBe("A/B/C-Wochen");
      expect(weekCycleLabel(4)).toBe("A/B/C/D-Wochen");
    });

    it("renders unsupported long cycles without enumerating every slot", () => {
      expect(weekCycleLabel(5)).toBe("5-Wochen-Zyklus");
      expect(weekCycleLabel(32_767)).toBe("32767-Wochen-Zyklus");
    });

    it("returns null for weekly repetition", () => {
      expect(weekCycleLabel(1)).toBeNull();
      expect(weekCycleLabel(0)).toBeNull();
    });
  });

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
