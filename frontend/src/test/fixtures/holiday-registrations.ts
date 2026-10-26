/** Fixtures of the review of the holiday registrations (#4663). */

import type { HolidayPeriod, TraegerSchool } from "~/lib/traeger-api";
import type {
  HolidayRegistrationDetail,
  HolidayRegistrationList,
} from "~/lib/traeger-holiday-registrations-api";
import type { HolidayContribution } from "~/lib/traeger-contributions-api";

export const HOLIDAY_SCHOOLS: TraegerSchool[] = [
  { id: "31", name: "OGS Nord", slug: "nord", subdomain: "nord", city: "" },
  { id: "32", name: "OGS Süd", slug: "sued", subdomain: "sued", city: "" },
];

export const HOLIDAY_PERIOD: HolidayPeriod = {
  id: "5",
  name: "Winterferien 2027",
  registrationOpensOn: "2026-09-01",
  registrationClosesOn: "2026-12-04",
  binding: true,
  notes: "",
  status: "published",
  publishedAt: "2026-08-20T10:00:00Z",
  publicToken: "link",
  registrationCount: 2,
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

export const HOLIDAY_REGISTRATION: HolidayRegistrationDetail = {
  id: "7",
  status: "received",
  frozen: false,
  submittedAt: "2026-09-03T08:00:00Z",
  guardianFirstName: "Eva",
  guardianLastName: "Muster",
  guardianEmail: "eva@example.org",
  guardianPhone: "0151 2345678",
  benefitsDeclared: false,
  children: [
    {
      id: "9",
      firstName: "Lia",
      lastName: "Muster",
      birthDate: "2018-03-01",
      homeSchoolId: "31",
      guestSchoolName: "",
      duplicateRegistrationIds: ["8"],
      bookings: [
        {
          weekId: "51",
          weekStartsOn: "2027-01-04",
          weekEndsOn: "2027-01-08",
          hostSchoolId: "31",
          timeModelId: "62",
          timeModelName: "bis 16 Uhr",
        },
      ],
    },
    {
      id: "10",
      firstName: "Ben",
      lastName: "Muster",
      birthDate: "2016-09-12",
      homeSchoolId: null,
      guestSchoolName: "Grundschule Melle",
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
    },
  ],
  health: [
    { childId: "9", allergies: "Erdnüsse", medication: "", healthNotes: "" },
  ],
  contacts: [
    {
      kind: "emergency",
      name: "Oma Muster",
      phone: "05402 1234",
      relationship: "Großmutter",
    },
  ],
  privacyConsentAt: "2026-09-03T08:00:00Z",
  healthConsentAt: "2026-09-03T08:00:00Z",
  bindingConfirmedAt: "2026-09-03T08:00:00Z",
  history: [],
};

export const HOLIDAY_REGISTRATION_LIST: HolidayRegistrationList = {
  registrations: [HOLIDAY_REGISTRATION],
  total: 2,
  statusCounts: { received: 1, confirmed: 0, cancelled: 1 },
};

/** The preview of HOLIDAY_REGISTRATION's contribution (#4664). */
export const HOLIDAY_CONTRIBUTION: HolidayContribution = {
  children: [
    {
      childId: "9",
      position: 1,
      totalCents: 6750,
      lines: [
        {
          kind: "base",
          amountCents: 6750,
          weekStartsOn: "2027-01-04",
          weekEndsOn: "2027-01-08",
          timeModelName: "bis 16 Uhr",
          childRank: null,
          discountBasisPoints: null,
          discountCents: null,
          note: "",
        },
      ],
    },
    {
      childId: "10",
      position: 2,
      totalCents: 2250,
      lines: [
        {
          kind: "base",
          amountCents: 4500,
          weekStartsOn: "2027-01-11",
          weekEndsOn: "2027-01-15",
          timeModelName: "bis 13 Uhr",
          childRank: null,
          discountBasisPoints: null,
          discountCents: null,
          note: "",
        },
        {
          kind: "sibling",
          amountCents: -2250,
          weekStartsOn: null,
          weekEndsOn: null,
          timeModelName: "",
          childRank: 2,
          discountBasisPoints: 5000,
          discountCents: null,
          note: "",
        },
      ],
    },
  ],
  totalCents: 9000,
  exemptionAvailable: true,
  benefitsExemption: false,
  frozen: false,
  confirmedAt: null,
};
