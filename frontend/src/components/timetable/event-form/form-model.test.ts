import { describe, expect, it } from "vitest";

import type { TimetableTemplate } from "~/lib/timetable-types";

import {
  emptyForm,
  formFromSeries,
  hasPerWeekdayStaffDeviation,
  parseMaxParticipants,
  sourceScopesOverlap,
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

  it("keeps offering sources and grade filters as dynamic rules", () => {
    const form = formFromSeries(
      template({
        targetGroupType: "angebot",
        sourceCareOfferingIds: ["41", "42"],
        sourceGradeLevels: [1, 3],
        studentIds: ["99"],
      }),
      "2026-08-13",
    );

    expect(form.targetGroupType).toBe("angebot");
    expect(form.sourceCareOfferingIds).toEqual(["41", "42"]);
    expect(form.sourceGradeLevels).toEqual([1, 3]);
    // Sourced children stay server-managed; the occurrence snapshot is not
    // promoted to a manually maintained static roster.
    expect(form.studentIds).toEqual([]);
  });

  it("seeds the stored Teilnehmergrenze and maps null to empty (#2233)", () => {
    expect(
      formFromSeries(template({ maxParticipants: 43 }), "2026-08-13")
        .maxParticipants,
    ).toBe("43");
    expect(
      formFromSeries(template({ maxParticipants: null }), "2026-08-13")
        .maxParticipants,
    ).toBe("");
    expect(emptyForm("2026-08-13").maxParticipants).toBe("");
  });
});

describe("parseMaxParticipants", () => {
  it("maps empty input to null (unbegrenzt)", () => {
    expect(parseMaxParticipants("")).toBeNull();
    expect(parseMaxParticipants("   ")).toBeNull();
  });

  it("accepts whole positive integers", () => {
    expect(parseMaxParticipants("43")).toBe(43);
    expect(parseMaxParticipants(" 1 ")).toBe(1);
  });

  it("rejects zero, negatives, fractions and junk", () => {
    expect(parseMaxParticipants("0")).toBeNull();
    expect(parseMaxParticipants("-5")).toBeNull();
    expect(parseMaxParticipants("2.5")).toBeNull();
    expect(parseMaxParticipants("1e2")).toBeNull();
    expect(parseMaxParticipants("abc")).toBeNull();
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

describe("formFromSeries — Quellenfilter-Modus (#2482)", () => {
  it("opens in Klasse mode when a class filter is stored", () => {
    const form = formFromSeries(
      template({
        targetGroupType: "angebot",
        sourceCareOfferingIds: ["7"],
        sourceSchoolClasses: ["1b"],
      }),
      "2026-08-03",
    );
    expect(form.sourceFilterMode).toBe("klasse");
    expect(form.sourceSchoolClasses).toEqual(["1b"]);
    expect(form.sourceGradeLevels).toEqual([]);
  });

  it("opens in Jahrgang mode when a grade filter is stored", () => {
    const form = formFromSeries(
      template({
        targetGroupType: "angebot",
        sourceCareOfferingIds: ["7"],
        sourceGradeLevels: [1],
      }),
      "2026-08-03",
    );
    expect(form.sourceFilterMode).toBe("jahrgang");
  });

  it("opens unfiltered when the source has no filter at all", () => {
    const form = formFromSeries(
      template({ targetGroupType: "angebot", sourceCareOfferingIds: ["7"] }),
      "2026-08-03",
    );
    expect(form.sourceFilterMode).toBe("alle");
  });
});

describe("sourceScopesOverlap (#2482)", () => {
  it("treats an empty filter as all children", () => {
    expect(sourceScopesOverlap([], [], [1], [])).toBe(true);
    expect(sourceScopesOverlap([2], [], [], [])).toBe(true);
  });

  it("finds no overlap between two different classes of one Jahrgang", () => {
    expect(sourceScopesOverlap([], ["1a"], [], ["1b"])).toBe(false);
    expect(sourceScopesOverlap([], ["1a"], [], ["1a", "1b"])).toBe(true);
  });

  it("finds the overlap between a class and its Jahrgang", () => {
    expect(sourceScopesOverlap([], ["1b"], [1], [])).toBe(true);
    expect(sourceScopesOverlap([], ["1b"], [2], [])).toBe(false);
  });

  it("compares Jahrgang filters directly", () => {
    expect(sourceScopesOverlap([1, 2], [], [2], [])).toBe(true);
    expect(sourceScopesOverlap([1], [], [3], [])).toBe(false);
  });
});
