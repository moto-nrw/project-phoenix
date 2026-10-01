import { describe, expect, it } from "vitest";
import { mondayWeekRows } from "./lazy-day-picker";

describe("mondayWeekRows", () => {
  it("counts the Monday-first week rows the calendar grid renders", () => {
    // Feb 2027 starts on a Monday and has 28 days: exactly four rows.
    expect(mondayWeekRows(new Date(2027, 1, 10))).toBe(4);
    // Sep 2026 starts on a Tuesday: one leading day, 30 days, five rows.
    expect(mondayWeekRows(new Date(2026, 8, 9))).toBe(5);
    // Mar 2026 starts on a Sunday: six leading days, 31 days, six rows.
    expect(mondayWeekRows(new Date(2026, 2, 29))).toBe(6);
  });
});
