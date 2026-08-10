import { describe, expect, it } from "vitest";
import {
  careOfferingAvailabilityReason,
  careOfferingAvailabilityRuleError,
  careOfferingIsAvailable,
  countCareOfferingRuleConflicts,
  describeCareOfferingAvailabilityRule,
  formatGradeLevelList,
} from "./care-offering-availability";
import type { CareOfferingAvailabilityRule } from "./care-offering-api";

const rule = (
  match: "all" | "any",
  operator: "in" | "not_in",
  value: number[],
): CareOfferingAvailabilityRule => ({
  match,
  conditions: [{ source: "grade_level", operator, value }],
});

describe("careOfferingIsAvailable", () => {
  it("keeps offerings without conditions universally available", () => {
    expect(careOfferingIsAvailable({}, undefined)).toBe(true);
    expect(
      careOfferingIsAvailable(
        { availability_rule: { match: "all", conditions: [] } },
        undefined,
      ),
    ).toBe(true);
  });

  it.each([
    [rule("all", "in", [1, 2]), "2", true],
    [rule("all", "in", [1, 2]), "3", false],
    [rule("all", "not_in", [3, 4]), "2", true],
    [rule("all", "not_in", [3, 4]), "3", false],
    [rule("all", "not_in", [3, 4]), "", false],
  ])("evaluates grade rules", (availabilityRule, grade, expected) => {
    expect(
      careOfferingIsAvailable({ availability_rule: availabilityRule }, grade),
    ).toBe(expected);
  });

  it("supports all and any across multiple conditions", () => {
    const conditions: CareOfferingAvailabilityRule["conditions"] = [
      { source: "grade_level", operator: "in", value: [2] },
      { source: "grade_level", operator: "not_in", value: [2] },
    ];
    expect(
      careOfferingIsAvailable(
        { availability_rule: { match: "all", conditions } },
        2,
      ),
    ).toBe(false);
    expect(
      careOfferingIsAvailable(
        { availability_rule: { match: "any", conditions } },
        2,
      ),
    ).toBe(true);
  });
});

describe("careOfferingAvailabilityRuleError", () => {
  it("identifies the invalid condition", () => {
    expect(careOfferingAvailabilityRuleError(rule("all", "in", []), 4)).toBe(
      "Bedingung 1: Wähle mindestens eine Klassenstufe.",
    );
    expect(
      careOfferingAvailabilityRuleError(rule("all", "in", [5]), 4),
    ).toContain("Bedingung 1");
  });
});

describe("formatGradeLevelList", () => {
  it("collapses contiguous runs and keeps gaps explicit", () => {
    expect(formatGradeLevelList([1, 2, 3])).toBe("1–3");
    expect(formatGradeLevelList([2])).toBe("2");
    expect(formatGradeLevelList([1, 3])).toBe("1 und 3");
    expect(formatGradeLevelList([1, 2, 4])).toBe("1–2 und 4");
    expect(formatGradeLevelList([1, 2, 4, 6])).toBe("1–2, 4 und 6");
  });

  it("sorts, de-duplicates and drops non-integers", () => {
    expect(formatGradeLevelList([3, 1, 2, 2])).toBe("1–3");
    expect(formatGradeLevelList([1.5, 2])).toBe("2");
  });

  it("returns an empty string for an empty list", () => {
    expect(formatGradeLevelList([])).toBe("");
  });
});

describe("describeCareOfferingAvailabilityRule", () => {
  it("returns null when nothing is restricted", () => {
    expect(describeCareOfferingAvailabilityRule(null)).toBeNull();
    expect(describeCareOfferingAvailabilityRule(undefined)).toBeNull();
    expect(
      describeCareOfferingAvailabilityRule({ match: "all", conditions: [] }),
    ).toBeNull();
  });

  it("phrases a single positive condition", () => {
    expect(
      describeCareOfferingAvailabilityRule(rule("all", "in", [1, 2])),
    ).toBe("Nur für Klassen 1–2");
    expect(describeCareOfferingAvailabilityRule(rule("all", "in", [3]))).toBe(
      "Nur für Klasse 3",
    );
  });

  it("phrases a single negative condition", () => {
    expect(
      describeCareOfferingAvailabilityRule(rule("all", "not_in", [4])),
    ).toBe("Nicht für Klasse 4");
    expect(
      describeCareOfferingAvailabilityRule(rule("all", "not_in", [3, 4])),
    ).toBe("Nicht für Klassen 3–4");
  });

  it("joins several conditions with the rule's own connective", () => {
    const conditions: CareOfferingAvailabilityRule["conditions"] = [
      { source: "grade_level", operator: "in", value: [1, 2] },
      { source: "grade_level", operator: "not_in", value: [2] },
    ];
    expect(
      describeCareOfferingAvailabilityRule({ match: "all", conditions }),
    ).toBe("Nur für Klassen 1–2 und nicht Klasse 2");
    expect(
      describeCareOfferingAvailabilityRule({ match: "any", conditions }),
    ).toBe("Nur für Klassen 1–2 oder nicht Klasse 2");
  });

  it("ignores conditions that describe nothing", () => {
    expect(
      describeCareOfferingAvailabilityRule(rule("all", "in", [])),
    ).toBeNull();
    expect(
      describeCareOfferingAvailabilityRule({
        match: "all",
        conditions: [
          {
            source: "unknown_source",
            operator: "in",
            value: [1],
          } as unknown as CareOfferingAvailabilityRule["conditions"][number],
        ],
      }),
    ).toBeNull();
  });
});

describe("careOfferingAvailabilityReason", () => {
  it("returns null for an offering the child may pick", () => {
    expect(careOfferingAvailabilityReason({}, "3")).toBeNull();
    expect(
      careOfferingAvailabilityReason(
        { availability_rule: rule("all", "in", [1, 2]) },
        "2",
      ),
    ).toBeNull();
  });

  it("names the rule and the child's grade — the support case", () => {
    expect(
      careOfferingAvailabilityReason(
        { availability_rule: rule("all", "in", [1, 2]) },
        "3",
      ),
    ).toBe("Nicht wählbar: nur für Klassen 1–2 (Kind: Klasse 3)");
  });

  it("says so when the child has no grade level yet", () => {
    expect(
      careOfferingAvailabilityReason(
        { availability_rule: rule("all", "in", [1, 2]) },
        "",
      ),
    ).toBe(
      "Nicht wählbar: nur für Klassen 1–2 (Klassenstufe des Kindes fehlt)",
    );
  });

  it("degrades to a bare reason when the rule cannot be phrased", () => {
    expect(
      careOfferingAvailabilityReason(
        { availability_rule: rule("all", "in", []) },
        "",
      ),
    ).toBe("Nicht wählbar (Klassenstufe des Kindes fehlt)");
  });
});

describe("countCareOfferingRuleConflicts", () => {
  const counts = {
    grade_levels: { "1": 12, "2": 5, "3": 1 },
    unknown_grade_count: 0,
  };

  it("counts nothing without a rule or without stats", () => {
    expect(countCareOfferingRuleConflicts(null, counts)).toBe(0);
    expect(
      countCareOfferingRuleConflicts({ match: "all", conditions: [] }, counts),
    ).toBe(0);
    expect(countCareOfferingRuleConflicts(rule("all", "in", [1]), null)).toBe(
      0,
    );
  });

  it("counts the bookings a positive rule excludes", () => {
    expect(
      countCareOfferingRuleConflicts(rule("all", "in", [1, 2]), counts),
    ).toBe(1);
    expect(countCareOfferingRuleConflicts(rule("all", "in", [1]), counts)).toBe(
      6,
    );
  });

  it("counts the bookings a negative rule excludes", () => {
    expect(
      countCareOfferingRuleConflicts(rule("all", "not_in", [1]), counts),
    ).toBe(12);
  });

  it("treats a booking without a grade level as conflicting", () => {
    expect(
      countCareOfferingRuleConflicts(rule("all", "in", [1, 2, 3]), {
        grade_levels: { "1": 4 },
        unknown_grade_count: 2,
      }),
    ).toBe(2);
  });
});
