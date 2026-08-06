import { describe, expect, it } from "vitest";

import type { TimetableTemplate } from "~/lib/timetable-types";

import {
  emptyForm,
  formFromSeries,
  hasPerWeekdayStaffDeviation,
} from "./form-model";

function template(
  overrides: Partial<TimetableTemplate> = {},
): TimetableTemplate {
  return {
    id: "1",
    name: "Mittagessen",
    type: "care",
    categoryId: "2",
    categoryName: "Betreuung",
    isOpen: true,
    maxParticipants: 30,
    targetGroupType: "none",
    enrollmentCount: 0,
    supervisorCount: 0,
    requiredStaffCount: 0,
    assignedStaffCount: 0,
    studentIds: [],
    staffIds: [],
    schedules: [],
    weekdayAssignments: [],
    ...overrides,
  };
}

describe("formFromSeries", () => {
  it("keeps an independent education group separate from dynamic targets", () => {
    const form = formFromSeries(
      template({ educationGroupId: "17", targetGroupType: "none" }),
      "2026-08-13",
    );

    expect(form.educationGroupId).toBe("17");
    expect(form.educationGroupIds).toEqual([]);
  });

  it("loads every dynamic education group without treating it as independent", () => {
    const form = formFromSeries(
      template({
        educationGroupId: "17",
        targetGroupType: "gruppe",
        targets: [
          { type: "gruppe", educationGroupId: "17" },
          { type: "gruppe", educationGroupId: "18" },
        ],
      }),
      "2026-08-13",
    );

    expect(form.educationGroupId).toBe("");
    expect(form.educationGroupIds).toEqual(["17", "18"]);
  });
});

describe("hasPerWeekdayStaffDeviation", () => {
  const perWeekdayForm = () => ({
    ...emptyForm("2026-08-13"),
    weekdays: [1, 2],
    perWeekdayRoster: true,
    weekdayRosters: {
      1: { staffIds: ["7"], primaryStaffId: "", studentIds: ["11"] },
      2: { staffIds: ["7"], primaryStaffId: "", studentIds: ["12"] },
    },
  });

  it("is false in shared mode and for uniform per-weekday staffing", () => {
    expect(hasPerWeekdayStaffDeviation(emptyForm("2026-08-13"))).toBe(false);
    // Differing child lists alone are no staffing deviation — a sourced
    // roster replaces the children by design.
    expect(hasPerWeekdayStaffDeviation(perWeekdayForm())).toBe(false);
  });

  it("detects a weekday staffed differently", () => {
    const form = perWeekdayForm();
    form.weekdayRosters[2] = {
      staffIds: ["8"],
      primaryStaffId: "",
      studentIds: [],
    };
    expect(hasPerWeekdayStaffDeviation(form)).toBe(true);
  });

  it("detects a diverging zuständige Person", () => {
    const form = perWeekdayForm();
    form.weekdayRosters[2] = {
      staffIds: ["7"],
      primaryStaffId: "7",
      studentIds: [],
    };
    expect(hasPerWeekdayStaffDeviation(form)).toBe(true);
  });
});
