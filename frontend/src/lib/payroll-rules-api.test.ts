import { describe, expect, it } from "vitest";

import {
  clockToMinutes,
  formatRate,
  mapPayrollRules,
  minutesToClock,
  parseRate,
  wageTypesForAbsences,
  wageTypesForRate,
  type PayrollWageType,
} from "./payroll-rules-api";

describe("payroll rule formats", () => {
  it("reads a rate with comma or point as hundredths", () => {
    expect(parseRate("25")).toBe(2500);
    expect(parseRate("25,5")).toBe(2550);
    expect(parseRate(" 2.50 ")).toBe(250);
    expect(parseRate("0,01")).toBe(1);
  });

  it("rejects rates DATEV cannot book", () => {
    expect(parseRate("")).toBeNull();
    expect(parseRate("0")).toBeNull();
    expect(parseRate("-5")).toBeNull();
    expect(parseRate("12,345")).toBeNull();
    expect(parseRate("1000,01")).toBeNull();
    expect(parseRate("abc")).toBeNull();
  });

  it("shows percent without zero cents and amounts with two places", () => {
    expect(formatRate("prozent", 2500)).toBe("25");
    expect(formatRate("prozent", 2550)).toBe("25,50");
    expect(formatRate("betrag", 250)).toBe("2,50");
  });

  it("converts wall-clock times both ways", () => {
    expect(minutesToClock(1200)).toBe("20:00");
    expect(minutesToClock(360)).toBe("06:00");
    expect(clockToMinutes("06:00")).toBe(360);
    expect(clockToMinutes("23:59")).toBe(1439);
    expect(clockToMinutes("24:00")).toBeNull();
    expect(clockToMinutes("6 Uhr")).toBeNull();
  });
});

describe("mapPayrollRules", () => {
  it("maps the backend wire onto the page model", () => {
    const rules = mapPayrollRules({
      wage_types: [
        { id: "7", name: "Nachtzuschlag", number: "310", unit: "stunden" },
      ],
      surcharge_rules: [
        {
          id: "8",
          kind: "nacht",
          window_start_minute: 1200,
          window_end_minute: 360,
          rate_type: "prozent",
          rate_hundredths: 2500,
          wage_type_id: "7",
        },
      ],
      absence_types: [
        { id: "9", name: "Sonderurlaub", is_active: false, wage_type_id: null },
      ],
    });

    expect(rules).toEqual({
      wageTypes: [
        { id: "7", name: "Nachtzuschlag", number: "310", unit: "stunden" },
      ],
      surchargeRules: [
        {
          id: "8",
          kind: "nacht",
          windowStartMinute: 1200,
          windowEndMinute: 360,
          rateType: "prozent",
          rateHundredths: 2500,
          wageTypeId: "7",
        },
      ],
      absenceTypes: [
        { id: "9", name: "Sonderurlaub", isActive: false, wageTypeId: null },
      ],
    });
  });
});

describe("wage type choices", () => {
  const wageTypes: PayrollWageType[] = [
    { id: "1", name: "Stunden", number: "", unit: "stunden" },
    { id: "2", name: "Tage", number: "", unit: "tage" },
    { id: "3", name: "Betrag", number: "", unit: "betrag" },
  ];

  it("offers a percent rule hour wage types and an amount rule amount wage types", () => {
    expect(wageTypesForRate(wageTypes, "prozent").map((w) => w.id)).toEqual([
      "1",
    ]);
    expect(wageTypesForRate(wageTypes, "betrag").map((w) => w.id)).toEqual([
      "3",
    ]);
  });

  it("offers an absence type hour and day wage types", () => {
    expect(wageTypesForAbsences(wageTypes).map((w) => w.id)).toEqual([
      "1",
      "2",
    ]);
  });
});
