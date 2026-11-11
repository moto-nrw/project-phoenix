import { describe, expect, it } from "vitest";

import { hostedWeeksInRange } from "./closing-day-holiday-hint";
import type { HostedHolidayWeek } from "~/lib/holiday-host-api";

const week = (
  id: string,
  startsOn: string,
  endsOn: string,
  participants: number,
): HostedHolidayWeek => ({
  id,
  periodName: "Sommerferien",
  startsOn,
  endsOn,
  participants,
  bookingUnit: "week",
  careStartsAt: "08:00",
  careEndsAt: "16:00",
  dayParticipants: new Map(),
  guestGroupId: null,
  guests: 0,
});

const weeks = [
  week("1", "2026-08-03", "2026-08-07", 12),
  week("2", "2026-08-10", "2026-08-14", 0),
  week("3", "2026-08-17", "2026-08-21", 4),
];

describe("hostedWeeksInRange (#6434)", () => {
  it("names only hosted weeks with children that the range touches", () => {
    expect(
      hostedWeeksInRange(weeks, {
        startDate: "2026-08-07",
        endDate: "2026-08-17",
        schoolIds: [],
      }).map((hit) => hit.id),
    ).toEqual(["1", "3"]);
    expect(
      hostedWeeksInRange(weeks, {
        startDate: "2026-08-10",
        endDate: "2026-08-14",
        schoolIds: [],
      }),
    ).toEqual([]);
  });
});
