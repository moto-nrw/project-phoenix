import { describe, expect, it } from "vitest";

import { mapDailyProjectionResponse } from "./time-tracking-helpers";

describe("mapDailyProjectionResponse target source (#3259)", () => {
  it("marks Sonderarbeitszeit days and leaves regular days unchanged", () => {
    const projection = mapDailyProjectionResponse([
      {
        date: "2026-10-21",
        target_minutes: 510,
        credit_minutes: 0,
        actual_minutes: 0,
        balance_minutes: -510,
        target_source: "override",
      },
      {
        date: "2026-10-26",
        target_minutes: 480,
        credit_minutes: 0,
        actual_minutes: 480,
        balance_minutes: 0,
      },
    ]);

    expect(projection.get("2026-10-21")?.isOverride).toBe(true);
    expect(projection.get("2026-10-26")).toEqual({
      targetMinutes: 480,
      creditMinutes: 0,
      actualMinutes: 480,
      balanceMinutes: 0,
    });
  });
});
