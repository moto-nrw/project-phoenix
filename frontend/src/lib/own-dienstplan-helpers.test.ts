import { describe, expect, it } from "vitest";

import type { StaffShift } from "~/lib/shift-helpers";
import type { DayProjection } from "~/lib/time-tracking-helpers";

import {
  calendarWeekDays,
  plannedWeekMinutes,
  shiftTypesFromShifts,
  targetWeekMinutes,
  visibleWeekDays,
} from "./own-dienstplan-helpers";

const WEEK = [
  "2026-09-07",
  "2026-09-08",
  "2026-09-09",
  "2026-09-10",
  "2026-09-11",
  "2026-09-12",
  "2026-09-13",
];

function shift(overrides: Partial<StaffShift> = {}): StaffShift {
  return {
    id: "11",
    staffId: "42",
    date: "2026-09-07",
    startTime: "08:00",
    endTime: "12:00",
    breakMinutes: 0,
    shiftTypeId: null,
    shiftTypeName: null,
    shiftTypeColor: null,
    notes: "",
    seriesId: null,
    detached: false,
    cancelled: false,
    changeReason: null,
    originShiftId: null,
    ...overrides,
  };
}

function day(targetMinutes: number): DayProjection {
  return {
    targetMinutes,
    creditMinutes: 0,
    actualMinutes: 0,
    balanceMinutes: 0,
  };
}

describe("calendarWeekDays", () => {
  it("returns Monday to Sunday of the week containing the day", () => {
    expect(calendarWeekDays("2026-09-09")).toEqual(WEEK);
    expect(calendarWeekDays("2026-09-13")).toEqual(WEEK);
  });

  it("stays on calendar days across the DST switch", () => {
    expect(calendarWeekDays("2026-10-25")).toEqual([
      "2026-10-19",
      "2026-10-20",
      "2026-10-21",
      "2026-10-22",
      "2026-10-23",
      "2026-10-24",
      "2026-10-25",
    ]);
  });
});

describe("visibleWeekDays", () => {
  it("shows Monday to Friday without weekend shifts", () => {
    expect(visibleWeekDays(WEEK, [shift()])).toEqual(WEEK.slice(0, 5));
  });

  it("adds Saturday for a Saturday shift", () => {
    expect(visibleWeekDays(WEEK, [shift({ date: "2026-09-12" })])).toEqual(
      WEEK.slice(0, 6),
    );
  });

  it("adds Saturday and Sunday for a Sunday shift", () => {
    expect(visibleWeekDays(WEEK, [shift({ date: "2026-09-13" })])).toEqual(
      WEEK,
    );
  });
});

describe("plannedWeekMinutes", () => {
  it("sums span minus break and skips cancelled shifts", () => {
    expect(
      plannedWeekMinutes([
        shift({ startTime: "08:00", endTime: "12:30", breakMinutes: 30 }),
        shift({ id: "12", startTime: "13:00", endTime: "15:00" }),
        shift({ id: "13", date: "2026-09-12", endTime: "10:00" }),
        shift({ id: "14", cancelled: true }),
      ]),
    ).toBe(240 + 120 + 120);
  });
});

describe("targetWeekMinutes", () => {
  it("sums the daily targets of the week", () => {
    const projection = new Map([
      ["2026-09-07", day(240)],
      ["2026-09-08", day(240)],
      // Feiertag: Soll 0 vom Server.
      ["2026-09-09", day(0)],
      ["2026-09-14", day(480)],
    ]);
    expect(targetWeekMinutes(WEEK, projection)).toBe(480);
  });

  it("is null without a projection for the week", () => {
    expect(targetWeekMinutes(WEEK, undefined)).toBeNull();
    expect(targetWeekMinutes(WEEK, new Map())).toBeNull();
  });
});

describe("shiftTypesFromShifts", () => {
  it("builds one type per id from the shift labels, sorted by name", () => {
    const types = shiftTypesFromShifts([
      shift({
        shiftTypeId: "21",
        shiftTypeName: "Spätdienst",
        shiftTypeColor: "#5080D8",
      }),
      shift({
        id: "12",
        shiftTypeId: "22",
        shiftTypeName: "Frühdienst",
        shiftTypeColor: "#83CD2D",
      }),
      shift({ id: "13", shiftTypeId: "21", shiftTypeName: "Spätdienst" }),
      shift({ id: "14" }),
    ]);
    expect(types.map((type) => [type.id, type.name, type.color])).toEqual([
      ["22", "Frühdienst", "#83CD2D"],
      ["21", "Spätdienst", "#5080D8"],
    ]);
  });
});
