import { describe, expect, it } from "vitest";
import type { CareTariffs } from "~/lib/traeger-care-billing-api";
import {
  addMonths,
  firstOpenMonth,
  formatMonth,
  missingPrices,
  monthOf,
  priceRowsAt,
  priceStands,
  shownWeekdays,
  standAt,
  weekdayList,
} from "./care-billing-format";

// The SJR pattern (#4684): Monday until 14:00, Tuesday until 16:30.
const data: CareTariffs = {
  schools: [{ id: "11", name: "Grundschule Ispringen" }],
  modules: [
    {
      schoolId: "11",
      schoolName: "Grundschule Ispringen",
      careOfferingId: "101",
      moduleName: "Betreuung bis 14:00 Uhr",
      endsAt: "14:00",
      bookedWeekdays: ["mon", "sat"],
      contractCount: 3,
    },
    {
      schoolId: "11",
      schoolName: "Grundschule Ispringen",
      careOfferingId: "102",
      moduleName: "Betreuung bis 16:30 Uhr",
      endsAt: "16:30",
      bookedWeekdays: ["tue"],
      contractCount: 1,
    },
  ],
  tariffs: [
    {
      id: "1",
      schoolId: "11",
      careOfferingId: "101",
      moduleName: "Betreuung bis 14:00 Uhr",
      weekday: "mon",
      validFrom: "2026-09-01",
      validUntil: "2027-01-31",
      monthlyCents: 3200,
    },
    {
      id: "2",
      schoolId: "11",
      careOfferingId: "101",
      moduleName: "Betreuung bis 14:00 Uhr",
      weekday: "mon",
      validFrom: "2027-02-01",
      validUntil: null,
      monthlyCents: 3600,
    },
    {
      id: "3",
      schoolId: "11",
      careOfferingId: "102",
      moduleName: "Betreuung bis 16:30 Uhr",
      weekday: "tue",
      validFrom: "2026-09-01",
      validUntil: null,
      monthlyCents: 4550,
    },
  ],
  billedUntil: { "11": "2026-12-01" },
};

describe("care billing months", () => {
  it("names and moves calendar months without a time zone", () => {
    expect(formatMonth("2027-01-01")).toBe("Januar 2027");
    expect(formatMonth("2026-03-01")).toBe("März 2026");
    expect(addMonths("2026-12-01", 1)).toBe("2027-01-01");
    expect(addMonths("2027-01-01", -1)).toBe("2026-12-01");
    expect(monthOf("2026-11-07")).toBe("2026-11-01");
  });
});

describe("care price matrix", () => {
  it("shows the price that holds at the month", () => {
    const january = priceRowsAt(data, "11", "2027-01-01");
    expect(january.map((row) => row.prices)).toEqual([
      { mon: 3200 },
      { tue: 4550 },
    ]);
    const february = priceRowsAt(data, "11", "2027-02-01");
    expect(february[0]?.prices).toEqual({ mon: 3600 });
  });

  it("names a booked weekday without a price", () => {
    const rows = priceRowsAt(data, "11", "2027-01-01");
    expect(rows[0] && missingPrices(rows[0])).toEqual(["sat"]);
    expect(shownWeekdays(rows)).toEqual([
      "mon",
      "tue",
      "wed",
      "thu",
      "fri",
      "sat",
    ]);
  });

  it("lists the price lists and picks the one in force", () => {
    const stands = priceStands(data, "11");
    expect(stands).toEqual(["2026-09-01", "2027-02-01"]);
    expect(standAt(stands, "2026-11-01")).toBe("2026-09-01");
    expect(standAt(stands, "2027-03-01")).toBe("2027-02-01");
    expect(standAt(stands, "2026-01-01")).toBe("2026-09-01");
    expect(standAt([], "2026-01-01")).toBe("");
  });

  it("lets a new price start after the last billed month", () => {
    expect(firstOpenMonth(data, "11", "2026-11-01")).toBe("2027-01-01");
    expect(firstOpenMonth(data, "12", "2026-11-01")).toBe("2026-11-01");
  });

  it("lists weekdays in their order", () => {
    expect(weekdayList(["fri", "mon", "tue"])).toBe("Mo, Di, Fr");
  });
});
