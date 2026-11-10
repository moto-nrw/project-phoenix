import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  ClosingDayHolidayHint,
  hostedWeeksInRange,
} from "./closing-day-holiday-hint";
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
  guestGroupId: null,
  guests: 0,
});

const weeks = [
  week("1", "2026-08-03", "2026-08-07", 12),
  week("2", "2026-08-10", "2026-08-14", 0),
  week("3", "2026-08-17", "2026-08-21", 4),
];

describe("ClosingDayHolidayHint (#6434)", () => {
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

  it("says who tells the parents and that the bookings stay", () => {
    render(
      <ClosingDayHolidayHint
        weeks={weeks}
        draft={{
          startDate: "2026-08-05",
          endDate: "2026-08-19",
          schoolIds: [],
        }}
      />,
    );
    expect(
      screen.getByText("In dieser Zeit sind Ferienwochen bei Ihnen gebucht"),
    ).toBeInTheDocument();
    expect(screen.getByText(/12 Kinder/)).toBeInTheDocument();
    expect(screen.getByText(/4 Kinder/)).toBeInTheDocument();
    expect(
      screen.getByText(
        "Der Träger sieht den Schließtag und informiert die Eltern. Die Buchungen bleiben bestehen.",
      ),
    ).toBeInTheDocument();
  });
});
