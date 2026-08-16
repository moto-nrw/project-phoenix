import { describe, expect, it } from "vitest";

import { formatDecimalHours, parseDecimalHours } from "./arbeitszeitmodell-tab";

describe("parseDecimalHours", () => {
  it.each([
    ["4,45", 267],
    ["4.45", 267],
    ["0,33", 20],
    ["0,025", 2],
    ["1,025", 62],
    [",5", 30],
    [".5", 30],
    [" 12,00 ", 720],
    ["0", 0],
  ])("converts %s to integer minutes", (value, minutes) => {
    expect(parseDecimalHours(value)).toEqual({ status: "valid", minutes });
  });

  it("treats an empty value as an empty work day", () => {
    expect(parseDecimalHours("  ")).toEqual({ status: "empty" });
  });

  it.each([
    "-0,01",
    "12,01",
    "12.0000000000000001",
    "13",
    "abc",
    "4,4,5",
    "1e2",
    "NaN",
    "Infinity",
  ])("rejects %s", (value) => {
    expect(parseDecimalHours(value)).toEqual({ status: "invalid" });
  });
});

describe("formatDecimalHours", () => {
  it.each([
    [1, "0,02"],
    [20, "0,33"],
    [59, "0,98"],
    [60, "1"],
    [267, "4,45"],
    [719, "11,98"],
    [720, "12"],
  ])("formats %i existing minutes as %s", (minutes, value) => {
    expect(formatDecimalHours(minutes)).toBe(value);
    expect(parseDecimalHours(value)).toEqual({ status: "valid", minutes });
  });
});
