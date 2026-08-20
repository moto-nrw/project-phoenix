import { describe, expect, it } from "vitest";

import { STANDARD_ABSENCE_OPTIONS } from "./use-absence-type-options";

describe("STANDARD_ABSENCE_OPTIONS", () => {
  it("marks every standard type as fixed so none looks editable", () => {
    for (const option of STANDARD_ABSENCE_OPTIONS) {
      expect(option.fixed).toBe(true);
    }
  });

  it("omits Freizeitausgleich, which stays manager-controlled", () => {
    expect(
      STANDARD_ABSENCE_OPTIONS.map((option) => option.value),
    ).not.toContain("comp_time");
  });
});
