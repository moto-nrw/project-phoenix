import { describe, expect, it } from "vitest";

import {
  NO_PLAN_FILTER,
  isPlanFilterActive,
  parsePlanFilterId,
  planFilterKey,
  planFilterQuery,
} from "./plan-filter";

describe("plan filter (#4766)", () => {
  it("accepts only positive ids from the URL", () => {
    expect(parsePlanFilterId("17")).toBe("17");
    for (const raw of [null, undefined, "", "0", "-3", "abc", "1.5", "07"]) {
      expect(parsePlanFilterId(raw)).toBeNull();
    }
  });

  it("keeps the unfiltered cache key and query unchanged", () => {
    expect(isPlanFilterActive(NO_PLAN_FILTER)).toBe(false);
    expect(planFilterKey(NO_PLAN_FILTER)).toBe("");
    expect(planFilterQuery(NO_PLAN_FILTER)).toEqual({});
  });

  it("names group and person in key and query", () => {
    const filter = { groupId: "5", staffId: "11" };
    expect(isPlanFilterActive(filter)).toBe(true);
    expect(planFilterKey(filter)).toBe("-g5-p11");
    expect(planFilterQuery(filter)).toEqual({ group_id: "5", staff_id: "11" });
    expect(planFilterQuery({ groupId: null, staffId: "11" })).toEqual({
      staff_id: "11",
    });
  });
});
