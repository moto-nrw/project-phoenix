import { describe, expect, it } from "vitest";
import { rosterPickupTimeLabel } from "./timetable-roster-helpers";

describe("rosterPickupTimeLabel", () => {
  it("hides intentionally redacted pickup times without reporting a load error", () => {
    expect(rosterPickupTimeLabel(null, false, true)).toBeNull();
  });

  it("keeps failed and missing pickup times distinct", () => {
    expect(rosterPickupTimeLabel(null, false)).toBe("Nicht geladen");
    expect(rosterPickupTimeLabel(null, true)).toBe("—");
  });
});
