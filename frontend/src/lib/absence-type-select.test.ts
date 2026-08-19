import { describe, expect, it } from "vitest";

import {
  absenceRequestFor,
  customOptionValue,
  selectValueFor,
  STANDARD_ABSENCE_OPTIONS,
} from "./absence-type-select";

describe("selectValueFor", () => {
  it("uses the canonical type for a standard absence", () => {
    expect(selectValueFor("sick", null)).toBe("sick");
    expect(selectValueFor("other", undefined)).toBe("other");
  });

  it("uses the school's own art when the row carries one", () => {
    expect(selectValueFor("other", "7")).toBe("custom:7");
  });
});

describe("absenceRequestFor", () => {
  it("passes a standard type straight through with no art", () => {
    expect(absenceRequestFor("vacation")).toEqual({
      absence_type: "vacation",
      absence_type_id: null,
    });
  });

  it("sends the art's id and the base type it inherits", () => {
    expect(absenceRequestFor(customOptionValue("12"))).toEqual({
      absence_type: "other",
      absence_type_id: 12,
    });
  });

  it("round-trips a stored absence back into the same request", () => {
    const stored = { absenceType: "other", absenceTypeId: "12" };
    const value = selectValueFor(stored.absenceType, stored.absenceTypeId);
    expect(absenceRequestFor(value)).toEqual({
      absence_type: "other",
      absence_type_id: 12,
    });
  });
});

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
