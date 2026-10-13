import { describe, expect, it } from "vitest";

import { hasDayPickupChange, type StudentTableDay } from "./student-table";

function day(
  pickup: Partial<StudentTableDay["pickup"]>,
  absence?: StudentTableDay["absence"],
): StudentTableDay {
  return {
    absence,
    arrival: { isException: false, isAbsent: false },
    pickup: { isException: false, ...pickup },
  };
}

describe("hasDayPickupChange (#3946)", () => {
  it("is true for a one-day exception with a time", () => {
    expect(
      hasDayPickupChange(day({ isException: true, pickupTime: "13:30" })),
    ).toBe(true);
  });

  it("is false for the weekly plan's time", () => {
    expect(hasDayPickupChange(day({ pickupTime: "16:00" }))).toBe(false);
  });

  it("is false for an exception without a time", () => {
    expect(hasDayPickupChange(day({ isException: true }))).toBe(false);
  });

  it("is false while an absence line replaces the times", () => {
    expect(
      hasDayPickupChange(
        day({ isException: true, pickupTime: "13:30" }, { label: "krank" }),
      ),
    ).toBe(false);
  });
});
