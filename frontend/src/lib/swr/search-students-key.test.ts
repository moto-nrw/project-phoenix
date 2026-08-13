import { describe, expect, it } from "vitest";

import {
  SEARCH_STUDENTS_ALL_GROUPS_KEY,
  SEARCH_STUDENTS_KEY_PREFIX,
  searchStudentsGroupScope,
  searchStudentsKeyTargetsGroup,
} from "./search-students-key";

describe("searchStudentsGroupScope", () => {
  it("uses the unfiltered scope for an empty selection", () => {
    expect(searchStudentsGroupScope([])).toBe("gall");
  });

  it("keeps the single-group scope unchanged", () => {
    expect(searchStudentsGroupScope(["7"])).toBe("g7");
  });

  it("joins several selected groups into one segment (#2218)", () => {
    expect(searchStudentsGroupScope(["7", "9"])).toBe("g7.9");
  });

  it("drops ids the backend would not filter on", () => {
    // Same normalization the backend applies: 0, negatives and non-numeric
    // values are no restriction at all.
    expect(searchStudentsGroupScope(["007", "0", "not-a-group"])).toBe("g7");
    expect(searchStudentsGroupScope(["0"])).toBe("gall");
  });

  it("collapses duplicates", () => {
    expect(searchStudentsGroupScope(["7", "007"])).toBe("g7");
  });
});

describe("searchStudentsKeyTargetsGroup", () => {
  const key = (scope: string) =>
    `tenant:${SEARCH_STUDENTS_KEY_PREFIX}${scope}-Anna-Lena--------`;

  it("matches the only selected group", () => {
    expect(searchStudentsKeyTargetsGroup(key("g7"), "7")).toBe(true);
  });

  it("matches every group of a multi-group view (#2218)", () => {
    // Missing the second id would leave that tab stale after a check-in there.
    expect(searchStudentsKeyTargetsGroup(key("g7.9"), "7")).toBe(true);
    expect(searchStudentsKeyTargetsGroup(key("g7.9"), "9")).toBe(true);
  });

  it("does not match a group outside the selection", () => {
    expect(searchStudentsKeyTargetsGroup(key("g7.9"), "8")).toBe(false);
  });

  it("compares ids whole, so 8 never matches 80", () => {
    expect(searchStudentsKeyTargetsGroup(key("g80"), "8")).toBe(false);
    expect(searchStudentsKeyTargetsGroup(key("g80.90"), "8")).toBe(false);
    expect(searchStudentsKeyTargetsGroup(key("g80.90"), "90")).toBe(true);
  });

  it("does not treat the unfiltered scope as a group match", () => {
    // An unfiltered view is refetched through SEARCH_STUDENTS_ALL_GROUPS_KEY
    // instead, which stays the broad path.
    expect(searchStudentsKeyTargetsGroup(key("gall"), "7")).toBe(false);
    expect(key("gall")).toContain(SEARCH_STUDENTS_ALL_GROUPS_KEY);
  });

  it("ignores keys of other caches", () => {
    expect(searchStudentsKeyTargetsGroup("tenant:ogs-students-7-", "7")).toBe(
      false,
    );
  });
});
