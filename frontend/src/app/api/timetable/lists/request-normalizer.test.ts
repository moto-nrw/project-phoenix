import { describe, it, expect } from "vitest";
import { normalizeSlotListRequestBody } from "./request-normalizer";

describe("normalizeSlotListRequestBody", () => {
  it("leaves non-object bodies untouched", () => {
    expect(normalizeSlotListRequestBody(null)).toBeNull();
    expect(normalizeSlotListRequestBody("x")).toBe("x");
    expect(normalizeSlotListRequestBody([1, 2])).toEqual([1, 2]);
  });

  it("passes string IDs through unchanged", () => {
    expect(
      normalizeSlotListRequestBody({
        date: "2026-07-22",
        instance_ids: ["12", "34"],
        group_ids: ["5"],
      }),
    ).toEqual({
      date: "2026-07-22",
      instance_ids: ["12", "34"],
      group_ids: ["5"],
    });
  });

  it("coerces numeric IDs to strings", () => {
    expect(
      normalizeSlotListRequestBody({
        instance_ids: [12, 34],
        group_ids: [5],
      }),
    ).toEqual({ instance_ids: ["12", "34"], group_ids: ["5"] });
  });

  it("preserves the omit-vs-empty distinction", () => {
    // Omitted field stays absent (backend reads it as "all").
    expect(normalizeSlotListRequestBody({ date: "2026-07-22" })).toEqual({
      date: "2026-07-22",
    });
    // Explicit empty array is preserved (backend reads it as "none").
    expect(normalizeSlotListRequestBody({ instance_ids: [] })).toEqual({
      instance_ids: [],
    });
  });

  it("does not throw on malformed input — the backend validates and returns 400", () => {
    // A non-array field is passed through untouched so the backend rejects it.
    expect(() =>
      normalizeSlotListRequestBody({ instance_ids: "not-an-array" }),
    ).not.toThrow();
    expect(
      normalizeSlotListRequestBody({ instance_ids: "not-an-array" }),
    ).toEqual({ instance_ids: "not-an-array" });

    // Invalid/out-of-range array elements are passed through, not rejected here.
    expect(() =>
      normalizeSlotListRequestBody({
        instance_ids: ["abc", "0", -5, 1.5, true],
      }),
    ).not.toThrow();
    expect(
      normalizeSlotListRequestBody({
        instance_ids: ["abc", "0", -5, 1.5, true],
      }),
    ).toEqual({ instance_ids: ["abc", "0", "-5", 1.5, true] });
  });
});
