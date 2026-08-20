import { describe, expect, it } from "vitest";

import {
  absenceRequestFor,
  customIdFromOptionValue,
  customOptionValue,
  selectValueFor,
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
      absence_type_id: "12",
    });
  });

  it("round-trips a stored absence back into the same request", () => {
    const stored = { absenceType: "other", absenceTypeId: "12" };
    const value = selectValueFor(stored.absenceType, stored.absenceTypeId);
    expect(absenceRequestFor(value)).toEqual({
      absence_type: "other",
      absence_type_id: "12",
    });
  });
});

describe("customIdFromOptionValue", () => {
  it("recovers the id an option value was built from", () => {
    expect(customIdFromOptionValue(customOptionValue("12"))).toBe("12");
  });
});
