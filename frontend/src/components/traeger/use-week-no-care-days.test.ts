import { describe, expect, it } from "vitest";

import type { HolidayPeriod } from "~/lib/traeger-api";
import type { TraegerCalendar } from "~/lib/traeger-closing-days";
import type { WeekDraft } from "./holiday-period-model";
import { noCareDaysOfWeek } from "./use-week-no-care-days";

const calendar: TraegerCalendar = {
  schools: [
    {
      id: "1",
      name: "OGS Pforzheim",
      federalStateCode: "DE-BW",
      federalStateName: "Baden-Württemberg",
    },
    {
      id: "2",
      name: "OGS Iserlohn",
      federalStateCode: "DE-NW",
      federalStateName: "Nordrhein-Westfalen",
    },
  ],
  closingDays: [
    {
      id: "9",
      startDate: "2027-05-28",
      endDate: "2027-05-28",
      reason: "Brückentag",
      schoolIds: ["1"],
    },
  ],
  regions: [
    {
      code: "DE-BW",
      name: "Baden-Württemberg",
      holidays: [{ date: "2027-05-27", name: "Fronleichnam" }],
    },
  ],
};

const week = (hostSchoolId: string, id?: string): WeekDraft => ({
  key: "w1",
  id,
  startsOn: "2027-05-24",
  endsOn: "2027-05-28",
  hostSchoolId,
  timeModels: [],
});

describe("noCareDaysOfWeek (#4702)", () => {
  it("names the holidays of the host school's state and the Träger's closing days for it", () => {
    expect(noCareDaysOfWeek(week("1"), calendar, undefined)).toEqual({
      days: [
        { date: "2027-05-27", kind: "holiday", name: "Fronleichnam" },
        { date: "2027-05-28", kind: "closing_day", name: "Brückentag" },
      ],
      keptAsStored: false,
    });
    expect(noCareDaysOfWeek(week("2"), calendar, undefined).days).toEqual([]);
  });

  it("adds the stored week's own closing days and keeps an unchanged stored week", () => {
    const stored = {
      weeks: [
        {
          id: "5",
          startsOn: "2027-05-24",
          endsOn: "2027-05-28",
          hostSchoolId: "2",
          timeModels: [],
          noCareDays: [
            {
              date: "2027-05-25",
              kind: "closing_day",
              name: "Pädagogischer Tag",
            },
          ],
        },
      ],
    } as unknown as HolidayPeriod;
    expect(noCareDaysOfWeek(week("2", "5"), calendar, stored)).toEqual({
      days: [
        { date: "2027-05-25", kind: "closing_day", name: "Pädagogischer Tag" },
      ],
      keptAsStored: true,
    });
    expect(
      noCareDaysOfWeek(
        { ...week("2", "5"), startsOn: "2027-05-26" },
        calendar,
        stored,
      ),
    ).toEqual({
      days: [],
      keptAsStored: false,
    });
  });
});
