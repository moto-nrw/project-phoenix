import { describe, expect, it } from "vitest";

import {
  DAY_STATUSES,
  countedDayStatus,
  resolvePresence,
} from "./presence-status";
import { LOCATION_COLORS, LOCATION_STATUSES } from "./location-helper";

const BINARY = { mode: "binary" } as const;
const DETAILED = { mode: "detailed", displayMode: "roomName" } as const;

describe("resolvePresence mixes the day planning in itself (#6572)", () => {
  it.each([BINARY, DETAILED])(
    "turns unplanned attendance into the hint below the location (%o)",
    (view) => {
      const presence = resolvePresence(
        {
          current_location: "Anwesend - Raum 101",
          day_planning_status: "comes_today",
          day_planning_reason: "unplanned_attendance",
          day_planning_label: "laut Plan nicht da",
        },
        view,
      );

      expect(presence.pill.label).not.toBe(LOCATION_STATUSES.NOT_ARRIVAL);
      expect(presence.hint).toEqual({
        status: "not_arrival",
        title: "laut Plan nicht da",
      });
    },
  );

  it.each([BINARY, DETAILED])(
    "replaces the location of a planned absent child at home (%o)",
    (view) => {
      const presence = resolvePresence(
        {
          current_location: "Zuhause",
          day_planning_status: "not_coming_today",
          day_planning_label: "Arzttermin",
        },
        view,
      );

      expect(presence.pill).toMatchObject({
        label: LOCATION_STATUSES.NOT_ARRIVAL,
        color: LOCATION_COLORS.NOT_ARRIVAL,
        title: "Arzttermin",
      });
      expect(presence.hint).toBeNull();
    },
  );

  it("reads an arrival exception without a time as not coming", () => {
    const presence = resolvePresence(
      {
        current_location: "Schule",
        arrival_is_exception: true,
        arrival_notes: "Oma holt ab",
      },
      BINARY,
    );

    expect(presence.pill).toMatchObject({
      label: LOCATION_STATUSES.NOT_ARRIVAL,
      title: "Oma holt ab",
    });
  });
});

describe("resolvePresence takes rank, text and time from the table", () => {
  it("lets the stronger day status win at home and carries only its title", () => {
    const presence = resolvePresence(
      {
        current_location: "Zuhause",
        sick: true,
        excused: true,
        arrival_is_exception: true,
        arrival_notes: "Arzttermin",
      },
      DETAILED,
    );

    expect(presence.pill).toEqual({
      label: LOCATION_STATUSES.SICK,
      color: LOCATION_COLORS.SICK,
      title: undefined,
      locationStatus: LOCATION_STATUSES.HOME,
      presenceState: undefined,
    });
  });

  it("puts the contradiction to the plan first among the hints", () => {
    const presence = resolvePresence(
      {
        current_location: "Schulhof",
        sick: true,
        class_trip: true,
        arrival_is_exception: true,
      },
      BINARY,
    );

    expect(presence.hint?.status).toBe("not_arrival");
  });

  it("uses the day status' own time and falls back to the location time", () => {
    expect(
      resolvePresence(
        {
          current_location: "Zuhause",
          class_trip: true,
          class_trip_since: "2026-09-09T05:30:00Z",
          location_since: "2026-09-08T14:00:00Z",
        },
        BINARY,
      ).since,
    ).toBe("07:30");
    expect(
      resolvePresence(
        {
          current_location: "Schule",
          excused: true,
          location_since: "2026-09-09T06:15:00Z",
        },
        DETAILED,
      ).since,
    ).toBe("08:15");
  });

  it("keeps the location's rule for „seit“ under a plan-only status", () => {
    expect(
      resolvePresence(
        {
          current_location: "Schule",
          location_since: "2026-09-09T06:15:00Z",
          arrival_is_exception: true,
        },
        DETAILED,
      ).since,
    ).toBeNull();
  });
});

describe("countedDayStatus follows the table's ranking", () => {
  it("counts the first set status in pill order", () => {
    const counted = DAY_STATUSES.filter((status) => status.counted);
    expect(counted.map((status) => status.key)).toEqual([
      "sick",
      "class_trip",
      "excused",
    ]);
    expect(
      countedDayStatus({
        current_location: "Zuhause",
        class_trip: true,
        excused: true,
      }),
    ).toBe("class_trip");
  });
});

describe("planReasonTitle (#6663)", () => {
  const sickAndNotComing = {
    current_location: "Zuhause",
    sick: true,
    arrival_is_exception: true,
    arrival_notes: "Arzttermin",
  } as const;

  it("keeps the plan's reason as tooltip on a replaced pill", () => {
    const presence = resolvePresence(sickAndNotComing, {
      ...DETAILED,
      planReasonTitle: true,
    });

    expect(presence.pill).toMatchObject({
      label: LOCATION_STATUSES.SICK,
      title: "Arzttermin",
    });
  });

  it("gives no tooltip without a plan behind the pill", () => {
    const presence = resolvePresence(
      { current_location: "Zuhause", sick: true },
      { ...DETAILED, planReasonTitle: true },
    );

    expect(presence.pill).toMatchObject({
      label: LOCATION_STATUSES.SICK,
      title: undefined,
    });
  });

  it("leaves the tooltip to the shown status in the binary view", () => {
    expect(resolvePresence(sickAndNotComing, BINARY).pill).toMatchObject({
      label: LOCATION_STATUSES.SICK,
      title: undefined,
    });
  });
});
