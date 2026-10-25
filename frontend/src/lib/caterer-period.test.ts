import { describe, expect, it } from "vitest";
import { defaultPeriod, periodDays } from "./caterer-period";

describe("caterer periods (#3873)", () => {
  it("covers tomorrow's weekday, this week and next week", () => {
    expect(periodDays("tomorrow", "2026-09-09")).toEqual(["2026-09-10"]);
    expect(periodDays("tomorrow", "2026-09-11")).toEqual(["2026-09-14"]);
    expect(periodDays("this_week", "2026-09-09")).toEqual([
      "2026-09-07",
      "2026-09-08",
      "2026-09-09",
      "2026-09-10",
      "2026-09-11",
    ]);
    expect(periodDays("next_week", "2026-09-13")[0]).toBe("2026-09-14");
  });

  it("opens on next week from Friday on", () => {
    expect(defaultPeriod("2026-09-09")).toBe("this_week");
    expect(defaultPeriod("2026-09-11")).toBe("next_week");
    expect(defaultPeriod("2026-09-13")).toBe("next_week");
  });
});
