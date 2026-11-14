// Mittagessen in der Ferienbetreuung (#5631) im Träger-Portal: das Angebot
// einer Woche, das Essen einer Umbuchung und die Position im Beitrag.

import { describe, expect, it } from "vitest";
import type { HolidayPeriod } from "~/lib/traeger-api";
import type { HolidayRegistrationChild } from "~/lib/traeger-holiday-registrations-api";
import { effectiveRevenue } from "./booking-accounts-format";
import { contributionLineLabel, lunchCountText } from "./contribution-format";
import {
  draftFromPeriod,
  emptyHolidayPeriod,
  type HolidayPeriodDraft,
  inputFromDraft,
  validateHolidayPeriod,
  weekLunchTotal,
} from "./holiday-period-model";
import {
  rebookingBookings,
  rebookingChanged,
  rebookingDraft,
  rebookingLunch,
} from "./holiday-rebooking-model";

function draftWithLunch(
  lunchPrice: string,
  offered = true,
): HolidayPeriodDraft {
  const draft = emptyHolidayPeriod();
  const week = draft.weeks[0]!;
  return {
    ...draft,
    name: "Sommerferien 2027",
    registrationOpensOn: "2027-04-01",
    registrationClosesOn: "2027-05-31",
    weeks: [
      {
        ...week,
        startsOn: "2027-07-12",
        endsOn: "2027-07-16",
        hostSchoolId: "31",
        lunchOffered: offered,
        lunchPrice,
        timeModels: [
          {
            ...week.timeModels[0]!,
            name: "bis 16 Uhr",
            startsAt: "07:45",
            endsAt: "16:00",
            price: "65",
          },
        ],
      },
    ],
  };
}

const period: HolidayPeriod = {
  id: "5",
  name: "Sommerferien",
  registrationOpensOn: "2027-04-01",
  registrationClosesOn: "2027-05-31",
  binding: true,
  holidayWishRule: false,
  notes: "",
  status: "published",
  publishedAt: "2027-03-20T10:00:00Z",
  publicToken: "link",
  registrationCount: 1,
  weeks: [
    {
      id: "51",
      startsOn: "2027-07-12",
      endsOn: "2027-07-16",
      hostSchoolId: "31",
      lunchPriceCents: 430,
      lunchBooked: true,
      timeModels: [
        {
          id: "61",
          name: "bis 16 Uhr",
          startsAt: "07:45",
          endsAt: "16:00",
          priceCents: 6500,
        },
      ],
    },
    {
      id: "52",
      startsOn: "2027-07-19",
      endsOn: "2027-07-23",
      hostSchoolId: "31",
      timeModels: [
        {
          id: "62",
          name: "bis 16 Uhr",
          startsAt: "07:45",
          endsAt: "16:00",
          priceCents: 6500,
        },
      ],
    },
  ],
};

function childWith(
  bookings: readonly { weekId: string; timeModelId: string; lunch: boolean }[],
): HolidayRegistrationChild {
  return {
    id: "9",
    firstName: "Mia",
    lastName: "Muster",
    birthDate: "2018-03-01",
    homeSchoolId: "31",
    guestSchoolName: "",
    duplicateRegistrationIds: [],
    holidayWish: "",
    holidayWishSource: "",
    holidayWishCheck: false,
    bookings: bookings.map((booking) => ({
      ...booking,
      weekStartsOn: "2027-07-12",
      weekEndsOn: "2027-07-16",
      hostSchoolId: "31",
      timeModelName: "bis 16 Uhr",
      placement: "placed" as const,
      waitlistPosition: null,
      promotedAt: null,
    })),
  };
}

describe("lunch offer of a holiday week", () => {
  it("sends the price of one lunch in cent and nothing without the offer", () => {
    expect(
      inputFromDraft(draftWithLunch("4,3")).weeks[0]?.lunchPriceCents,
    ).toBe(430);
    // The typed price stays in the draft while the offer is off, but the
    // week then offers no lunch.
    expect(
      inputFromDraft(draftWithLunch("4,30", false)).weeks[0]?.lunchPriceCents,
    ).toBeNull();
  });

  it("asks for a price only while lunch is offered", () => {
    expect(validateHolidayPeriod(draftWithLunch("4,30"))).toEqual({});
    expect(validateHolidayPeriod(draftWithLunch("", false))).toEqual({});
    expect(validateHolidayPeriod(draftWithLunch(""))).toEqual({
      "weeks.0.lunch_price_cents":
        "Bitte geben Sie den Preis für ein Essen in Euro ein, zum Beispiel 4,30.",
    });
    expect(
      Object.keys(validateHolidayPeriod(draftWithLunch("vier Euro"))),
    ).toEqual(["weeks.0.lunch_price_cents"]);
  });

  it("sums the lunches of a whole week from its care days", () => {
    const week = draftWithLunch("4,30").weeks[0]!;
    expect(weekLunchTotal(week, 5)).toEqual({ lunches: 5, cents: 2150 });
    expect(weekLunchTotal(week, 4)).toEqual({ lunches: 4, cents: 1720 });
    expect(weekLunchTotal(week, 0)).toBeNull();
    expect(weekLunchTotal(draftWithLunch("").weeks[0]!, 5)).toBeNull();
  });

  it("reads a stored offer and that a lunch is booked", () => {
    const [withLunch, without] = draftFromPeriod(period).weeks;
    expect(withLunch).toMatchObject({
      lunchOffered: true,
      lunchPrice: "4,30",
      lunchBooked: true,
    });
    expect(without).toMatchObject({
      lunchOffered: false,
      lunchPrice: "",
      lunchBooked: false,
    });
  });
});

describe("lunch of a rebooking", () => {
  const child = childWith([{ weekId: "51", timeModelId: "61", lunch: true }]);
  const draft = rebookingDraft(period, child);

  it("starts from the booked lunch and keeps it when nothing changes", () => {
    const lunch = rebookingLunch(child);
    expect(lunch).toEqual({ "51": true });
    const bookings = rebookingBookings(period, draft, {}, lunch);
    expect(bookings).toEqual([
      { weekId: "51", timeModelId: "61", lunch: true },
    ]);
    expect(rebookingChanged(child, bookings)).toBe(false);
  });

  it("counts a lunch taken off as a change", () => {
    const bookings = rebookingBookings(period, draft, {}, {});
    expect(bookings).toEqual([{ weekId: "51", timeModelId: "61" }]);
    expect(rebookingChanged(child, bookings)).toBe(true);
  });

  it("books no lunch in a week that offers none", () => {
    const both = { ...draft, "52": "62" };
    expect(
      rebookingBookings(period, both, {}, { "51": true, "52": true }),
    ).toEqual([
      { weekId: "51", timeModelId: "61", lunch: true },
      { weekId: "52", timeModelId: "62" },
    ]);
  });
});

describe("lunch on the contribution and its account", () => {
  const line = {
    kind: "meal" as const,
    amountCents: 2150,
    weekStartsOn: "2027-07-12",
    weekEndsOn: "2027-07-16",
    timeModelName: "",
    childRank: null,
    discountBasisPoints: null,
    discountCents: null,
    note: "",
    bookedDays: 5,
  };

  it("names the lunches and the price of one lunch", () => {
    expect(contributionLineLabel(line)).toContain(
      "Mittagessen, 5 Essen zu je 4,30 €",
    );
    expect(contributionLineLabel(line)).toContain("12.07.");
    expect(lunchCountText({ amountCents: 430, bookedDays: 1 })).toBe(
      "1 Essen zu 4,30 €",
    );
    // A Storno negates the amount; the price of one lunch stays positive.
    expect(lunchCountText({ amountCents: -2150, bookedDays: 5 })).toBe(
      "5 Essen zu je 4,30 €",
    );
    expect(lunchCountText({ amountCents: 2150, bookedDays: null })).toBe("");
  });

  it("books the holiday lunch like the holiday care without its own account", () => {
    const page = {
      standardAccount: "4000",
      costCenterRequired: false,
      schools: [],
      offerings: [],
      costCenters: [],
      revenueAccounts: [
        { scope: "holiday" as const, scopeId: "0", account: "4400" },
      ],
    };
    expect(effectiveRevenue(page, "holiday_meals")).toEqual({
      account: "4400",
      source: "holiday",
    });
    expect(
      effectiveRevenue(
        {
          ...page,
          revenueAccounts: [
            ...page.revenueAccounts,
            { scope: "holiday_meals" as const, scopeId: "0", account: "4410" },
          ],
        },
        "holiday_meals",
      ),
    ).toEqual({ account: "4410", source: "own" });
    expect(
      effectiveRevenue({ ...page, revenueAccounts: [] }, "holiday_meals"),
    ).toEqual({ account: "4000", source: "standard" });
  });
});
