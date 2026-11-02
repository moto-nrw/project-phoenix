import { describe, expect, it } from "vitest";

import { mapOfferingGaps } from "./offering-gap";

describe("mapOfferingGaps", () => {
  it("carries the Klassenfilter and the creation guard (#5343)", () => {
    const [gap] = mapOfferingGaps({
      gaps: [
        {
          offering: { id: 7, name: "Randstunde" },
          student_count: 3,
          school_classes: ["1a"],
          weekdays: [2],
          acknowledged: false,
          acknowledged_classes: [],
          new_series_classes: ["1a"],
          new_series_plans_covered: false,
        },
      ],
    });

    expect(gap?.offering).toEqual({ id: "7", name: "Randstunde" });
    expect(gap?.newSeriesClasses).toEqual(["1a"]);
    expect(gap?.newSeriesPlansCovered).toBe(false);
  });

  it("offers no creation when the answer says nothing about it", () => {
    const [gap] = mapOfferingGaps({
      gaps: [
        {
          offering: { id: 7, name: "Randstunde" },
          student_count: 3,
          acknowledged: false,
        },
      ],
    });

    expect(gap?.newSeriesClasses).toEqual([]);
    expect(gap?.newSeriesPlansCovered).toBe(true);
  });
});
