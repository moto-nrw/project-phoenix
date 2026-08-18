import { describe, expect, it } from "vitest";

import {
  materializeCareSelection,
  type MaterializableOffering,
} from "./care-offering-materialization";

function offering(
  overrides: Partial<MaterializableOffering> & { id: string },
): MaterializableOffering {
  return {
    days_of_week_mode: "parent_choice",
    available_days: ["mon", "tue", "wed", "thu", "fri"],
    is_required: false,
    includes_lunch: false,
    ...overrides,
  };
}

describe("materializeCareSelection", () => {
  it("adds a triggered offering with the trigger's selected days", () => {
    const randstunde = offering({ id: "1" });
    const ganztag = offering({
      id: "2",
      auto_add_trigger_offering_ids: ["1"],
    });

    const result = materializeCareSelection(
      {
        gradeLevel: "2",
        offeringIds: ["1"],
        offeringDays: { "1": ["mon", "wed"] },
      },
      [randstunde, ganztag],
    );

    expect(result.offeringIds.has("2")).toBe(true);
    expect([...(result.automaticDays["2"] ?? [])].sort()).toEqual([
      "mon",
      "wed",
    ]);
    expect([...(result.autoAddContributors["2"] ?? [])]).toEqual(["1"]);
  });

  it("intersects trigger days with the target's available days", () => {
    const trigger = offering({ id: "1" });
    const target = offering({
      id: "2",
      available_days: ["mon", "tue"],
      auto_add_trigger_offering_ids: ["1"],
    });

    const result = materializeCareSelection(
      {
        gradeLevel: "",
        offeringIds: ["1"],
        offeringDays: { "1": ["mon", "fri"] },
      },
      [trigger, target],
    );

    expect([...(result.automaticDays["2"] ?? [])]).toEqual(["mon"]);
  });

  it("uses a fixed trigger's available days", () => {
    const trigger = offering({
      id: "1",
      days_of_week_mode: "fixed",
      available_days: ["tue", "thu"],
    });
    const target = offering({ id: "2", auto_add_trigger_offering_ids: ["1"] });

    const result = materializeCareSelection(
      { gradeLevel: "", offeringIds: ["1"], offeringDays: {} },
      [trigger, target],
    );

    expect([...(result.automaticDays["2"] ?? [])].sort()).toEqual([
      "thu",
      "tue",
    ]);
  });

  it("cascades chained rules to a fixed point", () => {
    const a = offering({ id: "1" });
    const b = offering({ id: "2", auto_add_trigger_offering_ids: ["1"] });
    const c = offering({ id: "3", auto_add_trigger_offering_ids: ["2"] });

    const result = materializeCareSelection(
      { gradeLevel: "", offeringIds: ["1"], offeringDays: { "1": ["fri"] } },
      [c, b, a],
    );

    expect(result.offeringIds.has("2")).toBe(true);
    expect(result.offeringIds.has("3")).toBe(true);
    expect([...(result.automaticDays["3"] ?? [])]).toEqual(["fri"]);
    expect([...(result.autoAddContributors["3"] ?? [])]).toEqual(["2"]);
  });

  it("skips a rule whose grade condition the child does not meet", () => {
    const trigger = offering({ id: "1" });
    const target = offering({
      id: "2",
      auto_add_trigger_offering_ids: ["1"],
      auto_add_grade_levels: [3, 4],
    });

    const result = materializeCareSelection(
      { gradeLevel: "2", offeringIds: ["1"], offeringDays: { "1": ["mon"] } },
      [trigger, target],
    );

    expect(result.offeringIds.has("2")).toBe(false);
    expect(result.automaticDays["2"]).toBeUndefined();
  });

  it("derives required-lunch days from care offerings without naming a trigger", () => {
    const care = offering({ id: "1", counts_as_care: true });
    const lunch = offering({
      id: "2",
      is_required: true,
      includes_lunch: true,
    });

    const result = materializeCareSelection(
      {
        gradeLevel: "",
        offeringIds: ["1"],
        offeringDays: { "1": ["mon", "tue"] },
      },
      [care, lunch],
    );

    expect(result.offeringIds.has("2")).toBe(true);
    expect([...(result.automaticDays["2"] ?? [])].sort()).toEqual([
      "mon",
      "tue",
    ]);
    expect(result.autoAddContributors["2"]?.size).toBe(0);
  });

  it("suppresses the lunch derivation when the server grade gate is false", () => {
    const care = offering({ id: "1", counts_as_care: true });
    const lunch = offering({
      id: "2",
      is_required: true,
      includes_lunch: true,
      auto_add_applies: false,
    });

    const result = materializeCareSelection(
      { gradeLevel: "", offeringIds: ["1"], offeringDays: { "1": ["mon"] } },
      [care, lunch],
    );

    expect(result.offeringIds.has("2")).toBe(false);
    expect(result.automaticDays["2"]).toBeUndefined();
  });

  it("lets the server grade gate override a mismatching client grade check", () => {
    const trigger = offering({ id: "1" });
    const target = offering({
      id: "2",
      auto_add_trigger_offering_ids: ["1"],
      auto_add_grade_levels: [3],
      auto_add_applies: true,
    });

    const result = materializeCareSelection(
      { gradeLevel: "2", offeringIds: ["1"], offeringDays: { "1": ["mon"] } },
      [trigger, target],
    );

    expect([...(result.automaticDays["2"] ?? [])]).toEqual(["mon"]);
  });

  it("keeps manually picked days alongside automatic ones", () => {
    const trigger = offering({ id: "1" });
    const target = offering({ id: "2", auto_add_trigger_offering_ids: ["1"] });

    const result = materializeCareSelection(
      {
        gradeLevel: "",
        offeringIds: ["1", "2"],
        offeringDays: { "1": ["mon"], "2": ["fri"] },
      },
      [trigger, target],
    );

    expect([...(result.offeringDays["2"] ?? [])].sort()).toEqual([
      "fri",
      "mon",
    ]);
    expect([...(result.automaticDays["2"] ?? [])]).toEqual(["mon"]);
  });
});
