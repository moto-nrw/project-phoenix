import { describe, expect, it } from "vitest";
import type { HolidayPeriod } from "~/lib/traeger-api";
import type { HolidayRegistrationChild } from "~/lib/traeger-holiday-registrations-api";
import {
  rebookingBookings,
  rebookingChanged,
  rebookingDraft,
} from "./holiday-rebooking-model";

const period: HolidayPeriod = {
  id: "5",
  name: "Winterferien",
  registrationOpensOn: "2026-11-02",
  registrationClosesOn: "2026-12-04",
  binding: true,
  notes: "",
  status: "published",
  publishedAt: "2026-10-20T10:00:00Z",
  publicToken: "link",
  registrationCount: 1,
  weeks: [
    {
      id: "51",
      startsOn: "2027-01-04",
      endsOn: "2027-01-08",
      hostSchoolId: "31",
      timeModels: [
        {
          id: "61",
          name: "bis 13 Uhr",
          startsAt: "08:00",
          endsAt: "13:00",
          priceCents: 4500,
        },
        {
          id: "62",
          name: "bis 16 Uhr",
          startsAt: "08:00",
          endsAt: "16:00",
          priceCents: 6750,
        },
      ],
    },
    {
      id: "52",
      startsOn: "2027-01-11",
      endsOn: "2027-01-15",
      hostSchoolId: "32",
      timeModels: [
        {
          id: "63",
          name: "bis 13 Uhr",
          startsAt: "08:00",
          endsAt: "13:00",
          priceCents: 4500,
        },
      ],
    },
  ],
};

const child: HolidayRegistrationChild = {
  id: "9",
  firstName: "Ben",
  lastName: "Muster",
  birthDate: "2016-09-12",
  homeSchoolId: null,
  guestSchoolName: "",
  duplicateRegistrationIds: [],
  bookings: [
    {
      weekId: "52",
      weekStartsOn: "2027-01-11",
      weekEndsOn: "2027-01-15",
      hostSchoolId: "32",
      timeModelId: "63",
      timeModelName: "bis 13 Uhr",
    },
  ],
};

describe("holiday rebooking model", () => {
  it("starts with the child's bookings and no choice for the other weeks", () => {
    expect(rebookingDraft(period, child)).toEqual({ "51": "", "52": "63" });
  });

  it("books the chosen weeks in the order of the period", () => {
    expect(rebookingBookings(period, { "52": "63", "51": "62" })).toEqual([
      { weekId: "51", timeModelId: "62" },
      { weekId: "52", timeModelId: "63" },
    ]);
    expect(rebookingBookings(period, { "51": "", "52": "" })).toEqual([]);
  });

  it("tells an unchanged draft from a rebooking", () => {
    expect(rebookingChanged(child, [{ weekId: "52", timeModelId: "63" }])).toBe(
      false,
    );
    expect(rebookingChanged(child, [{ weekId: "52", timeModelId: "61" }])).toBe(
      true,
    );
    expect(
      rebookingChanged(child, [
        { weekId: "51", timeModelId: "61" },
        { weekId: "52", timeModelId: "63" },
      ]),
    ).toBe(true);
  });
});
