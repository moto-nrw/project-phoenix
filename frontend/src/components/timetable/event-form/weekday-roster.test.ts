import { describe, expect, it } from "vitest";

import {
  changePerWeekdayRosterMode,
  emptyForm,
  formFromSeries,
  rosterForWeekday,
  seedWeekdayRosters,
  weekdayDeviatesFromBaseline,
  type EventFormState,
} from "./form-model";
import type { TimetableTemplate } from "~/lib/timetable-types";

// Issue #2129: one Regeltermin, different responsible people and children per
// weekday. These cover the pure state rules the wizard step builds on.

function seriesForm(overrides: Partial<EventFormState> = {}): EventFormState {
  return {
    ...emptyForm("2026-04-20", "1", "weekly"),
    weekdays: [1, 2, 3],
    staffIds: ["10", "11"],
    primaryStaffId: "10",
    studentIds: ["100"],
    ...overrides,
  };
}

describe("per-weekday roster state (#2129)", () => {
  it("falls back to the shared roster while per-weekday staffing is off", () => {
    const form = seriesForm({
      weekdayRosters: {
        2: { staffIds: ["99"], primaryStaffId: "", studentIds: [] },
      },
    });

    expect(rosterForWeekday(form, 2)).toEqual({
      staffIds: ["10", "11"],
      primaryStaffId: "10",
      studentIds: ["100"],
    });
  });

  it("seeds every weekday from the shared roster when switching the mode on", () => {
    const form = seriesForm();

    const seeded = seedWeekdayRosters(form, form.weekdays);

    expect(Object.keys(seeded)).toEqual(["1", "2", "3"]);
    for (const weekday of form.weekdays) {
      expect(seeded[weekday]).toEqual({
        staffIds: ["10", "11"],
        primaryStaffId: "10",
        studentIds: ["100"],
      });
    }
  });

  it("keeps protected children only on their enrollment weekdays", () => {
    const form = seriesForm({ studentIds: ["100", "200"] });

    const changed = changePerWeekdayRosterMode(form, true, 1, [
      { weekday: 1, studentIds: ["200"] },
      { weekday: 2, studentIds: ["300"] },
    ]);

    expect(changed.weekdayRosters[1]?.studentIds).toEqual(["100", "200"]);
    expect(changed.weekdayRosters[2]?.studentIds).toEqual(["100", "300"]);
    expect(changed.weekdayRosters[3]?.studentIds).toEqual(["100"]);
  });

  it("does not restore stale weekday rosters after editing the shared list", () => {
    const form = seriesForm({
      perWeekdayRoster: true,
      weekdayRosters: {
        1: { staffIds: ["10"], primaryStaffId: "10", studentIds: ["100"] },
        2: { staffIds: ["20"], primaryStaffId: "20", studentIds: ["200"] },
        3: { staffIds: ["30"], primaryStaffId: "30", studentIds: ["300"] },
      },
    });

    const shared = changePerWeekdayRosterMode(form, false, 1, []);
    expect(shared.weekdayRosters).toEqual({});

    const edited = {
      ...shared,
      staffIds: ["40"],
      primaryStaffId: "40",
      studentIds: ["400"],
    };
    const changed = changePerWeekdayRosterMode(edited, true, 1, []);

    for (const weekday of changed.weekdays) {
      expect(changed.weekdayRosters[weekday]).toEqual({
        staffIds: ["40"],
        primaryStaffId: "40",
        studentIds: ["400"],
      });
    }
  });

  it("seeds an added weekday from an existing day instead of the aggregate", () => {
    const form = seriesForm({
      weekdays: [1, 2],
      studentIds: ["100", "200"],
      perWeekdayRoster: true,
      weekdayRosters: {
        1: { staffIds: ["10"], primaryStaffId: "10", studentIds: ["100"] },
        2: { staffIds: ["20"], primaryStaffId: "20", studentIds: ["200"] },
      },
    });

    const seeded = seedWeekdayRosters(form, [1, 2, 3]);

    expect(seeded[2]).toEqual({
      staffIds: ["20"],
      primaryStaffId: "20",
      studentIds: ["200"],
    });
    expect(seeded[3]).toEqual({
      staffIds: ["10"],
      primaryStaffId: "10",
      studentIds: ["100"],
    });
  });

  it("flags only the weekdays whose roster differs from the first weekday", () => {
    const form = seriesForm({
      perWeekdayRoster: true,
      weekdayRosters: {
        1: { staffIds: ["10"], primaryStaffId: "10", studentIds: ["100"] },
        2: { staffIds: ["20"], primaryStaffId: "20", studentIds: ["100"] },
        3: { staffIds: ["10"], primaryStaffId: "10", studentIds: ["100"] },
      },
    });

    expect(weekdayDeviatesFromBaseline(form, 1, form.weekdays)).toBe(false);
    expect(weekdayDeviatesFromBaseline(form, 2, form.weekdays)).toBe(true);
    expect(weekdayDeviatesFromBaseline(form, 3, form.weekdays)).toBe(false);
  });

  it("treats a different child list as a deviation too", () => {
    const form = seriesForm({
      perWeekdayRoster: true,
      weekdayRosters: {
        1: { staffIds: ["10"], primaryStaffId: "10", studentIds: ["100"] },
        2: {
          staffIds: ["10"],
          primaryStaffId: "10",
          studentIds: ["100", "101"],
        },
        3: { staffIds: ["10"], primaryStaffId: "10", studentIds: ["100"] },
      },
    });

    expect(weekdayDeviatesFromBaseline(form, 2, form.weekdays)).toBe(true);
  });

  it("reports no deviation while the shared mode is active", () => {
    const form = seriesForm({
      weekdayRosters: {
        2: { staffIds: ["99"], primaryStaffId: "", studentIds: [] },
      },
    });

    expect(weekdayDeviatesFromBaseline(form, 2, form.weekdays)).toBe(false);
  });
});

describe("reopening a series with per-weekday rosters", () => {
  const template = {
    id: "7",
    name: "Lernzeit",
    type: "care",
    categoryId: "3",
    categoryName: "Betreuung",
    isOpen: true,
    maxParticipants: 20,
    targetGroupType: "none",
    enrollmentCount: 2,
    supervisorCount: 2,
    requiredStaffCount: 1,
    assignedStaffCount: 1,
    studentIds: ["100", "200"],
    staffIds: ["10", "20"],
    schedules: [
      {
        id: "1",
        weekday: 1,
        startTime: "14:00",
        endTime: "15:00",
        weekPattern: 0,
      },
      {
        id: "2",
        weekday: 2,
        startTime: "14:00",
        endTime: "15:00",
        weekPattern: 0,
      },
    ],
    weekdayAssignments: [
      {
        weekday: 1,
        staffIds: ["10"],
        studentIds: ["100"],
        primaryStaffId: "10",
      },
      {
        weekday: 2,
        staffIds: ["20"],
        studentIds: ["200"],
        primaryStaffId: "20",
      },
    ],
  } satisfies TimetableTemplate;

  it("reopens in per-weekday mode so saving again cannot flatten the series", () => {
    const form = formFromSeries(template, "2026-04-20", "1");

    expect(form.perWeekdayRoster).toBe(true);
    expect(rosterForWeekday(form, 1)).toEqual({
      staffIds: ["10"],
      primaryStaffId: "10",
      studentIds: ["100"],
    });
    expect(rosterForWeekday(form, 2)).toEqual({
      staffIds: ["20"],
      primaryStaffId: "20",
      studentIds: ["200"],
    });
  });

  it("reopens a series without deviations in shared mode", () => {
    const form = formFromSeries(
      { ...template, weekdayAssignments: [] },
      "2026-04-20",
      "1",
    );

    expect(form.perWeekdayRoster).toBe(false);
    expect(rosterForWeekday(form, 2).staffIds).toEqual(["10", "20"]);
  });
});
