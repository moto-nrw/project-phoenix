import { describe, expect, it } from "vitest";
import {
  holidayCareDays,
  holidayDayList,
  holidayDayPromotions,
} from "./holiday-days";

describe("holiday days (#4701)", () => {
  it("takes Monday to Friday of a week as its care days", () => {
    expect(holidayCareDays("2027-01-04", "2027-01-10")).toEqual([
      "2027-01-04",
      "2027-01-05",
      "2027-01-06",
      "2027-01-07",
      "2027-01-08",
    ]);
    expect(holidayCareDays("2027-01-05", "2027-01-08")).toHaveLength(4);
  });

  it("names the days in order", () => {
    expect(holidayDayList(["2027-01-06", "2027-01-04"])).toBe(
      "Mo 04.01., Mi 06.01.",
    );
  });

  it("moves up a waiting booking only when all its days are free", () => {
    const placed = new Map([["2027-01-04", 1]]);
    expect(
      holidayDayPromotions(1, placed, [
        ["2027-01-04", "2027-01-05"],
        ["2027-01-05"],
        ["2027-01-05", "2027-01-06"],
        ["2027-01-06"],
      ]),
    ).toBe(2);
    expect(holidayDayPromotions(2, placed, [["2027-01-04"]])).toBe(1);
    expect(holidayDayPromotions(null, placed, [["2027-01-04"], []])).toBe(2);
  });
});
