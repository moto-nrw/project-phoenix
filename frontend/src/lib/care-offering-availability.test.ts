import { describe, expect, it } from "vitest";
import {
  careOfferingAvailabilityRuleError,
  careOfferingIsAvailable,
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
