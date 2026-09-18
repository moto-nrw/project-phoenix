import { describe, expect, it } from "vitest";

import { swrConfig } from "./config";
import { swrDataEqual } from "./compare";

describe("swrDataEqual", () => {
  it("tells maps with different values apart (#3258)", () => {
    const before = new Map([["2026-09-16", { creditMinutes: 0 }]]);
    const after = new Map([["2026-09-16", { creditMinutes: 480 }]]);

    expect(swrDataEqual(before, after)).toBe(false);
    expect(swrConfig.compare?.(before, after)).toBe(false);
  });

  it("treats maps with the same entries as equal", () => {
    expect(
      swrDataEqual(
        new Map([["a", { value: [1, 2] }]]),
        new Map([["a", { value: [1, 2] }]]),
      ),
    ).toBe(true);
    expect(swrDataEqual(new Map([["a", 1]]), new Map([["b", 1]]))).toBe(false);
    expect(swrDataEqual(new Map([["a", 1]]), new Map())).toBe(false);
  });

  it("compares sets by content", () => {
    expect(swrDataEqual(new Set(["a"]), new Set(["a"]))).toBe(true);
    expect(swrDataEqual(new Set(["a"]), new Set(["b"]))).toBe(false);
  });

  it("keeps the deep compare for plain data", () => {
    expect(
      swrDataEqual(
        { rows: [{ id: "1", at: new Date(0) }], total: 1 },
        { rows: [{ id: "1", at: new Date(0) }], total: 1 },
      ),
    ).toBe(true);
    expect(swrDataEqual({ total: 1 }, { total: 2 })).toBe(false);
    expect(swrDataEqual({ total: 1 }, { total: 1, extra: true })).toBe(false);
    expect(swrDataEqual([1, 2], [1, 2, 3])).toBe(false);
    expect(swrDataEqual(new Date(0), new Date(1))).toBe(false);
    expect(swrDataEqual(undefined, null)).toBe(false);
    expect(swrDataEqual(Number.NaN, Number.NaN)).toBe(true);
  });

  it("does not equate a map with a plain object", () => {
    expect(swrDataEqual(new Map(), {})).toBe(false);
    expect(swrDataEqual([], {})).toBe(false);
  });
});
