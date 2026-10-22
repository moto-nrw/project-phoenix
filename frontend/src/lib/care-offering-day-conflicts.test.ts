import { describe, expect, it } from "vitest";
import {
  AT_MOST_ONE_PER_DAY,
  careOfferingDayConflicts,
  conflictOfferingList,
  type DayRuleOffering,
} from "./care-offering-day-conflicts";

const variant = (
  id: string,
  mode: "fixed" | "parent_choice",
  days: string[],
  group = "Betreuungszeit",
): DayRuleOffering => ({
  id,
  days_of_week_mode: mode,
  available_days: days,
  selection_group: group,
  selection_rule: AT_MOST_ONE_PER_DAY,
});

const WEEK = ["mon", "tue", "wed", "thu", "fri"];
const short = variant("short", "parent_choice", WEEK);
const long = variant("long", "parent_choice", WEEK);
const fixedFriday = variant("friday", "fixed", ["fri"]);

describe("careOfferingDayConflicts", () => {
  it("allows different variants on different days", () => {
    expect(
      careOfferingDayConflicts([short, long], new Set(["short", "long"]), {
        short: ["mon"],
        long: ["tue"],
      }),
    ).toEqual([]);
  });

  it("names every shared weekday with its offerings", () => {
    expect(
      careOfferingDayConflicts([short, long], new Set(["short", "long"]), {
        short: new Set(["mon", "wed"]),
        long: new Set(["wed", "mon"]),
      }),
    ).toEqual([
      { group: "Betreuungszeit", day: "mon", offeringIds: ["short", "long"] },
      { group: "Betreuungszeit", day: "wed", offeringIds: ["short", "long"] },
    ]);
  });

  it("counts every day of a fixed offering", () => {
    expect(
      careOfferingDayConflicts(
        [short, fixedFriday],
        new Set(["short", "friday"]),
        { short: ["fri"] },
      ),
    ).toEqual([
      { group: "Betreuungszeit", day: "fri", offeringIds: ["short", "friday"] },
    ]);
  });

  it("ignores unselected offerings, other groups and other rules", () => {
    const course = variant("course", "parent_choice", WEEK, "Kurse");
    const atMostOne = { ...long, selection_rule: "at_most_one" };
    expect(
      careOfferingDayConflicts(
        [short, long, course],
        new Set(["short", "course"]),
        { short: ["mon"], long: ["mon"], course: ["mon"] },
      ),
    ).toEqual([]);
    expect(
      careOfferingDayConflicts([short, atMostOne], new Set(["short", "long"]), {
        short: ["mon"],
        long: ["mon"],
      }),
    ).toEqual([]);
  });
});

describe("conflictOfferingList", () => {
  it("quotes and joins the names the way the language does", () => {
    expect(conflictOfferingList(["bis 13 Uhr", "bis 17 Uhr"], "de")).toBe(
      "„bis 13 Uhr“ und „bis 17 Uhr“",
    );
    expect(conflictOfferingList(["A", "B", "C"], "en")).toBe(
      "“A”, “B”, and “C”",
    );
  });
});
