import { describe, expect, it } from "vitest";

import type { HostedHolidayWeek } from "~/lib/holiday-host-api";

import {
  holidayCareDays,
  holidayCareLabel,
  holidayCareSentence,
} from "./holiday-care";

function week(overrides: Partial<HostedHolidayWeek> = {}): HostedHolidayWeek {
  return {
    id: "3",
    periodName: "Winterferien 2027",
    startsOn: "2027-01-04",
    endsOn: "2027-01-08",
    participants: 3,
    bookingUnit: "week",
    careStartsAt: "07:45",
    careEndsAt: "16:00",
    dayParticipants: new Map(),
    guestGroupId: null,
    guests: 0,
    ...overrides,
  };
}

describe("holidayCareDays", () => {
  it("marks Monday to Friday of a hosted week with its children and care window", () => {
    const days = holidayCareDays([week()], "2027-01-04", "2027-01-10");

    expect([...days.keys()]).toEqual([
      "2027-01-04",
      "2027-01-05",
      "2027-01-06",
      "2027-01-07",
      "2027-01-08",
    ]);
    expect(days.get("2027-01-06")).toEqual({
      weekId: "3",
      children: 3,
      startsAt: "07:45",
      endsAt: "16:00",
    });
  });

  it("leaves out a closing day: the host school gives no care on it", () => {
    const days = holidayCareDays(
      [week()],
      "2027-01-04",
      "2027-01-08",
      new Map([["2027-01-06", "Pädagogischer Tag"]]),
    );

    expect(days.has("2027-01-05")).toBe(true);
    expect(days.has("2027-01-06")).toBe(false);
  });

  it("counts each day of a week booked by days on its own, 0 without a booking", () => {
    const days = holidayCareDays(
      [
        week({
          bookingUnit: "day",
          participants: 2,
          dayParticipants: new Map([
            ["2027-01-04", 2],
            ["2027-01-05", 1],
          ]),
        }),
      ],
      "2027-01-04",
      "2027-01-08",
    );

    expect(days.get("2027-01-04")?.children).toBe(2);
    expect(days.get("2027-01-05")?.children).toBe(1);
    expect(days.get("2027-01-06")?.children).toBe(0);
  });

  it("keeps to the visible window", () => {
    const days = holidayCareDays([week()], "2027-01-07", "2027-01-31");

    expect([...days.keys()]).toEqual(["2027-01-07", "2027-01-08"]);
    expect(holidayCareDays([week()], "2027-02-01", "2027-02-28").size).toBe(0);
  });
});

describe("holiday care texts", () => {
  const day = { weekId: "3", children: 3, startsAt: "07:45", endsAt: "16:00" };

  it("names the care, the children and the care window", () => {
    expect(holidayCareLabel(day)).toBe(
      "Ferienbetreuung · 3 Kinder · 07:45–16:00",
    );
    expect(
      holidayCareLabel({ ...day, children: 1 }, { withWindow: false }),
    ).toBe("Ferienbetreuung · 1 Kind");
    expect(holidayCareSentence(day)).toBe(
      "Ferienbetreuung für den Träger: 3 Kinder, 07:45 bis 16:00 Uhr.",
    );
  });

  it("says when no child is confirmed yet", () => {
    expect(holidayCareSentence({ ...day, children: 0 })).toBe(
      "Ferienbetreuung für den Träger: noch kein Kind bestätigt, 07:45 bis 16:00 Uhr.",
    );
  });
});
