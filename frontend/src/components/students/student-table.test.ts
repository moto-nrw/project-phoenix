import { describe, expect, it } from "vitest";

import { dayHomeChange, type StudentTableDay } from "./student-table";

function day(
  pickup: Partial<StudentTableDay["pickup"]>,
  options: {
    absence?: StudentTableDay["absence"];
    dayDepartureMode?: StudentTableDay["dayDepartureMode"];
  } = {},
): StudentTableDay {
  return {
    absence: options.absence,
    arrival: { isException: false, isAbsent: false },
    pickup: { isException: false, ...pickup },
    dayDepartureMode: options.dayDepartureMode,
  };
}

describe("dayHomeChange (#3946)", () => {
  it("names a one-day Gehzeit with a time", () => {
    expect(
      dayHomeChange(day({ isException: true, pickupTime: "13:30" })),
    ).toEqual({ time: true });
  });

  it("names a one-day Abholart", () => {
    expect(
      dayHomeChange(day({ pickupTime: "16:00" }, { dayDepartureMode: "bus" })),
    ).toEqual({ time: false, mode: "bus" });
  });

  it("names both when the Gehzeit and the Abholart changed", () => {
    expect(
      dayHomeChange(
        day(
          { isException: true, pickupTime: "13:30" },
          { dayDepartureMode: "pickup" },
        ),
      ),
    ).toEqual({ time: true, mode: "pickup" });
  });

  it("is empty for the weekly plan", () => {
    expect(dayHomeChange(day({ pickupTime: "16:00" }))).toBeUndefined();
  });

  it("is empty for a Gehzeit exception without a time", () => {
    expect(dayHomeChange(day({ isException: true }))).toBeUndefined();
  });

  it("is empty while an absence line replaces the times", () => {
    expect(
      dayHomeChange(
        day(
          { isException: true, pickupTime: "13:30" },
          { absence: { label: "krank" }, dayDepartureMode: "pickup" },
        ),
      ),
    ).toBeUndefined();
  });
});
