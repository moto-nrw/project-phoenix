import type {
  HolidayAllocationBoard,
  HolidayAllocationEntry,
  HolidayAllocationWeek,
} from "~/lib/traeger-holiday-allocation-api";

/** One open booking of the allocation list (#4700). */
export function allocationEntry(
  overrides: Partial<HolidayAllocationEntry> = {},
): HolidayAllocationEntry {
  return {
    bookingId: "100",
    rank: 1,
    registrationId: "7",
    submittedAt: "2026-09-01T08:00:00Z",
    childId: "9",
    childFirstName: "Mia",
    childLastName: "Muster",
    timeModelName: "bis 13 Uhr",
    days: [],
    criteria: [
      { criterionId: "1", value: "yes", source: "parent" },
      { criterionId: "2", value: "no", source: "parent" },
    ],
    proposal: "placed",
    ...overrides,
  };
}

/** A week with a capacity of two and three open bookings. */
export function allocationWeek(
  overrides: Partial<HolidayAllocationWeek> = {},
): HolidayAllocationWeek {
  return {
    weekId: "51",
    startsOn: "2027-07-05",
    endsOn: "2027-07-09",
    hostSchoolId: "31",
    bookingUnit: "week",
    capacity: 2,
    placed: 0,
    free: 2,
    pending: [
      allocationEntry(),
      allocationEntry({
        bookingId: "101",
        rank: 2,
        registrationId: "8",
        childId: "10",
        childFirstName: "Ben",
        childLastName: "Beispiel",
        criteria: [
          { criterionId: "1", value: "yes", source: "office" },
          { criterionId: "2", value: "unknown", source: "parent" },
        ],
      }),
      allocationEntry({
        bookingId: "102",
        rank: 3,
        registrationId: "8",
        childId: "11",
        childFirstName: "Lea",
        childLastName: "Beispiel",
        criteria: [
          { criterionId: "1", value: "no", source: "parent" },
          { criterionId: "2", value: "no", source: "parent" },
        ],
        proposal: "waitlisted",
      }),
    ],
    ...overrides,
  };
}

export function allocationBoard(
  overrides: Partial<HolidayAllocationBoard> = {},
): HolidayAllocationBoard {
  return {
    criteria: [
      {
        id: "1",
        name: "Beide Eltern berufstätig",
        scope: "family",
        position: 1,
      },
      {
        id: "2",
        name: "Geschwisterkind ist schon angemeldet",
        scope: "child",
        position: 2,
      },
    ],
    weeks: [allocationWeek()],
    decisions: [],
    ...overrides,
  };
}
