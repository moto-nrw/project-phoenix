import { describe, expect, it } from "vitest";

import type { TimetableTemplate } from "~/lib/timetable-types";

import { formFromSeries } from "./form-model";

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
