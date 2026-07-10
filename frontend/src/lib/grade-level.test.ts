import { describe, expect, it } from "vitest";
import { isSupportedGradeLevelMax } from "./grade-level";

describe("isSupportedGradeLevelMax", () => {
  it.each([1, 4, 13])("accepts supported integer cap %s", (value) => {
    expect(isSupportedGradeLevelMax(value)).toBe(true);
  });

  it.each([undefined, null, 0, 14, 4.5, "13"])(
    "rejects invalid cap %s instead of implying a fallback",
    (value) => {
      expect(isSupportedGradeLevelMax(value)).toBe(false);
    },
  );
});
