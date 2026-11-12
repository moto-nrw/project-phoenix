import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { PickupException } from "~/lib/pickup-schedule-helpers";

import {
  PickupChangeAlert,
  upcomingPickupChanges,
} from "./pickup-change-alert";

const TODAY = "2026-09-09";

function exception(over: Partial<PickupException>): PickupException {
  return {
    id: "1",
    studentId: "7",
    exceptionDate: TODAY,
    pickupTime: "14:30",
    source: "guardian",
    createdBy: "0",
    createdAt: "2026-09-08T10:00:00Z",
    updatedAt: "2026-09-08T10:00:00Z",
    ...over,
  };
}

describe("upcomingPickupChanges", () => {
  it("keeps changed days from today on, sorted, and drops the rest", () => {
    const changes = upcomingPickupChanges(
      [
        exception({
          id: "2",
          exceptionDate: "2026-09-11",
          pickupTime: "15:00:00",
        }),
        exception({ id: "1" }),
        exception({ id: "3", exceptionDate: "2026-09-08" }),
        exception({ id: "4", exceptionDate: "2026-09-10", source: "staff" }),
        exception({
          id: "6",
          exceptionDate: "2026-09-13",
          pickupTime: undefined,
          source: "staff",
        }),
        exception({
          id: "5",
          exceptionDate: "2026-09-12",
          pickupTime: undefined,
        }),
      ],
      TODAY,
    );

    expect(changes).toEqual([
      { date: TODAY, time: "14:30" },
      { date: "2026-09-10", time: "14:30" },
      { date: "2026-09-11", time: "15:00" },
    ]);
  });
});

describe("PickupChangeAlert", () => {
  it("names the day and the new time", () => {
    render(
      <PickupChangeAlert
        exceptions={[exception({ exceptionDate: "2026-09-10" })]}
        today={TODAY}
      />,
    );

    expect(
      screen.getByText("Abholzeit an einzelnen Tagen geändert"),
    ).toBeInTheDocument();
    expect(screen.getByText("Morgen: 14:30 Uhr")).toBeInTheDocument();
  });

  it("lists several days with their weekday", () => {
    render(
      <PickupChangeAlert
        exceptions={[
          exception({}),
          exception({
            id: "2",
            exceptionDate: "2026-09-14",
            pickupTime: "13:00",
          }),
        ]}
        today={TODAY}
      />,
    );

    expect(screen.getByText("Heute: 14:30 Uhr")).toBeInTheDocument();
    expect(
      screen.getByText("Montag, 14.09.2026: 13:00 Uhr"),
    ).toBeInTheDocument();
  });

  it("shows nothing once the change is withdrawn or past", () => {
    const { container } = render(
      <PickupChangeAlert
        exceptions={[exception({ exceptionDate: "2026-09-08" })]}
        today={TODAY}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });
});
