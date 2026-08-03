import { describe, expect, it } from "vitest";

import { reconcileCategoryId } from "./use-event-form";

describe("reconcileCategoryId", () => {
  const categories = [{ id: "1" }, { id: "2" }];

  it("keeps a selection that remains available", () => {
    expect(reconcileCategoryId("2", categories)).toBe("2");
  });

  it("clears a selection removed by archiving", () => {
    expect(reconcileCategoryId("3", categories)).toBe("");
  });

  it("keeps a newly created selection even if the refetch is stale", () => {
    expect(reconcileCategoryId("1", categories, "3")).toBe("3");
  });
});
