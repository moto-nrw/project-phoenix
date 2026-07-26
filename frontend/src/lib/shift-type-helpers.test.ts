import { describe, expect, it } from "vitest";

import { indexShiftTypes, mapShiftType } from "./shift-type-helpers";

describe("mapShiftType", () => {
  it("maps snake_case backend shape to camelCase with string id", () => {
    expect(
      mapShiftType({
        id: 5,
        name: "Betreuung",
        color: "#83CD2D",
        description: "Zeit am Kind",
        is_active: true,
      }),
    ).toEqual({
      id: "5",
      name: "Betreuung",
      color: "#83CD2D",
      description: "Zeit am Kind",
      isActive: true,
    });
  });

  it("defaults a missing description to an empty string", () => {
    expect(
      mapShiftType({ id: 1, name: "Pause", color: "#6B7280", is_active: false })
        .description,
    ).toBe("");
  });
});

describe("indexShiftTypes", () => {
  it("builds an id -> type lookup", () => {
    const types = [
      {
        id: "5",
        name: "Betreuung",
        color: "#83CD2D",
        description: "",
        isActive: true,
      },
      {
        id: "6",
        name: "Pause",
        color: "#6B7280",
        description: "",
        isActive: true,
      },
    ];
    const byId = indexShiftTypes(types);
    expect(byId.get("5")?.name).toBe("Betreuung");
    expect(byId.get("6")?.color).toBe("#6B7280");
    expect(byId.get("99")).toBeUndefined();
  });
});
