import { describe, expect, it } from "vitest";
import {
  careOfferingAvailabilityReason,
  careOfferingAvailabilityRuleError,
  careOfferingRuleExcludesNobody,
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

  // Describes what the rule EVALUATES to, not what its conditions say. The
  // old phrase-joining ("Klassen 1–2 und nicht Klasse 2") named grades the
  // rule rejects (#2186 review).
  it("names the grades a multi-condition rule actually admits", () => {
    expect(
      describeCareOfferingAvailabilityRule({
        match: "all",
        conditions: [
          { source: "grade_level", operator: "in", value: [1, 2] },
          { source: "grade_level", operator: "not_in", value: [2] },
        ],
      }),
    ).toBe("Nur für Klasse 1");
    expect(
      describeCareOfferingAvailabilityRule({
        match: "any",
        conditions: [
          { source: "grade_level", operator: "in", value: [1] },
          { source: "grade_level", operator: "in", value: [3] },
        ],
      }),
    ).toBe("Nur für Klassen 1 und 3");
  });

  // A rule no grade satisfies is a misconfiguration; it must not read like an
  // ordinary restriction.
  it("says so when a rule admits nobody", () => {
    expect(
      describeCareOfferingAvailabilityRule({
        match: "all",
        conditions: [
          { source: "grade_level", operator: "in", value: [1] },
          { source: "grade_level", operator: "in", value: [2] },
        ],
      }),
    ).toBe("Für keine Klassenstufe verfügbar");
  });

  it("prefers whichever side names fewer grades", () => {
    // Excluding one grade beats listing the other twelve.
    expect(
      describeCareOfferingAvailabilityRule(rule("all", "not_in", [4])),
    ).toBe("Nicht für Klasse 4");
    // With a tenant maximum of 6, "nur 1–4" is shorter stated negatively.
    expect(
      describeCareOfferingAvailabilityRule(rule("all", "in", [1, 2, 3, 4]), 6),
    ).toBe("Nicht für Klassen 5–6");
  });

  it("stays silent for a rule that turns nobody away", () => {
    expect(
      describeCareOfferingAvailabilityRule({
        match: "any",
        conditions: [
          { source: "grade_level", operator: "not_in", value: [1] },
          { source: "grade_level", operator: "not_in", value: [2] },
        ],
      }),
    ).toBeNull();
    expect(
      describeCareOfferingAvailabilityRule(rule("all", "in", [1, 2, 3, 4]), 4),
    ).toBeNull();
  });

  it("stays silent for a rule it cannot read", () => {
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

describe("careOfferingRuleExcludesNobody", () => {
  it("treats an absent or empty rule as no restriction", () => {
    expect(careOfferingRuleExcludesNobody(null)).toBe(true);
    expect(
      careOfferingRuleExcludesNobody({ match: "all", conditions: [] }),
    ).toBe(true);
  });

  it("detects a real restriction", () => {
    expect(careOfferingRuleExcludesNobody(rule("all", "in", [1, 2]))).toBe(
      false,
    );
    expect(careOfferingRuleExcludesNobody(rule("all", "not_in", [4]))).toBe(
      false,
    );
  });

  // The reviewer's case: two disjoint not_in conditions under match "any" are
  // satisfied by every grade, because a grade failing one satisfies the other.
  it("detects a tautological any-rule", () => {
    expect(
      careOfferingRuleExcludesNobody({
        match: "any",
        conditions: [
          { source: "grade_level", operator: "not_in", value: [1] },
          { source: "grade_level", operator: "not_in", value: [2] },
        ],
      }),
    ).toBe(true);
  });

  it("detects a rule that simply lists every grade the school has", () => {
    const allFour = rule("all", "in", [1, 2, 3, 4]);
    expect(careOfferingRuleExcludesNobody(allFour, 4)).toBe(true);
    // Without the tenant maximum it still restricts grades 5..13.
    expect(careOfferingRuleExcludesNobody(allFour)).toBe(false);
  });

  it("falls back to the supported range for a nonsensical maximum", () => {
    expect(careOfferingRuleExcludesNobody(rule("all", "in", [1, 2]), 0)).toBe(
      false,
    );
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

  // #2186 review: without the tenant ceiling the helper reasons over grades
  // 1..13, so it can phrase the restriction around grades the school has no
  // idea about. A two-grade school is told which grade MAY attend.
  it("honours the tenant grade ceiling", () => {
    const offering = { availability_rule: rule("all", "not_in", [1]) };
    expect(careOfferingAvailabilityReason(offering, "1", 2)).toBe(
      "Nicht wählbar: nur für Klasse 2 (Kind: Klasse 1)",
    );
    // Without it grades 2..13 all count as allowed, so the phrasing flips to
    // the exclusion instead.
    expect(careOfferingAvailabilityReason(offering, "1")).toBe(
      "Nicht wählbar: nicht für Klasse 1 (Kind: Klasse 1)",
    );
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
