import { describe, expect, it } from "vitest";

import type { TraegerCalendar } from "~/lib/traeger-closing-days";
import {
  calendarDayMarks,
  closingDaySchoolsLabel,
  dayCountLabel,
  isoDaysBetween,
} from "./closing-day-format";

const schools = [
  {
    id: "1",
    name: "OGS Nord",
    federalStateCode: "DE-BW",
    federalStateName: "Baden-Württemberg",
  },
  {
    id: "2",
    name: "OGS Süd",
    federalStateCode: "DE-NW",
    federalStateName: "Nordrhein-Westfalen",
  },
];

describe("closing day format (#4702)", () => {
  it("expands a range into its days, across a month end", () => {
    expect(isoDaysBetween("2027-04-30", "2027-05-02")).toEqual([
      "2027-04-30",
      "2027-05-01",
      "2027-05-02",
    ]);
    expect(
      dayCountLabel({ startDate: "2027-05-07", endDate: "2027-05-07" }),
    ).toBe("1 Tag");
    expect(
      dayCountLabel({ startDate: "2027-05-07", endDate: "2027-05-10" }),
    ).toBe("4 Tage");
  });

  it("names all schools or the chosen ones", () => {
    expect(closingDaySchoolsLabel({ schoolIds: ["1", "2"] }, schools)).toBe(
      "Alle Schulen",
    );
    expect(closingDaySchoolsLabel({ schoolIds: ["2"] }, schools)).toBe(
      "OGS Süd",
    );
  });

  it("marks holidays over closing days and names the state when there are several", () => {
    const calendar: TraegerCalendar = {
      schools,
      closingDays: [
        {
          id: "9",
          startDate: "2027-05-27",
          endDate: "2027-05-28",
          reason: "Brückentag",
          schoolIds: ["2"],
        },
      ],
      regions: [
        {
          code: "DE-BW",
          name: "Baden-Württemberg",
          holidays: [{ date: "2027-05-27", name: "Fronleichnam" }],
        },
        {
          code: "DE-NW",
          name: "Nordrhein-Westfalen",
          holidays: [{ date: "2027-05-27", name: "Fronleichnam" }],
        },
      ],
    };
    const marks = calendarDayMarks(calendar);
    expect(marks.get("2027-05-27")?.kind).toBe("holiday");
    expect(marks.get("2027-05-27")?.labels).toEqual([
      "Feiertag in Nordrhein-Westfalen: Fronleichnam",
      "Feiertag in Baden-Württemberg: Fronleichnam",
      "Schließtag: Brückentag",
    ]);
    expect(marks.get("2027-05-28")).toEqual({
      kind: "closing",
      labels: ["Schließtag: Brückentag"],
    });
    expect(marks.has("2027-05-29")).toBe(false);
  });
});
