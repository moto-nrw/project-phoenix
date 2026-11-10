import { fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.generated";
import type { HolidayRegistrationOffer } from "~/lib/holiday-registration-api";
import { HOLIDAY_CONSENT_TEXTS } from "~/test/fixtures/holiday-consent-texts";
import type { ChildDraft } from "./holiday-registration-model";
import { ChildCard } from "./holiday-registration-sections";

const week = {
  startsOn: "2027-01-04",
  endsOn: "2027-01-08",
  bookingUnit: "day" as const,
  careDays: ["2027-01-04", "2027-01-05"],
};
const offer: HolidayRegistrationOffer = {
  traegerName: "Träger Beispiel",
  name: "Winterferien",
  registrationOpensOn: "2026-09-01",
  registrationClosesOn: "2026-12-04",
  binding: true,
  notes: "",
  open: true,
  captchaSiteKey: "site",
  consentTexts: HOLIDAY_CONSENT_TEXTS,
  form: { version: 1, fields: [] },
  maxDaysPerChild: 1,
  weeks: [
    {
      ...week,
      id: "11",
      hostSchoolName: "OGS Nord",
      timeModels: [
        {
          id: "21",
          name: "Vormittag",
          startsAt: "08:00",
          endsAt: "13:00",
          priceCents: 1500,
        },
      ],
    },
    {
      ...week,
      id: "12",
      hostSchoolName: "OGS Süd",
      timeModels: [
        {
          id: "22",
          name: "Vormittag",
          startsAt: "08:00",
          endsAt: "13:00",
          priceCents: 1500,
        },
      ],
    },
  ],
  schools: [],
};

const child: ChildDraft = {
  key: "child",
  firstName: "Ben",
  lastName: "Muster",
  birthDate: "2018-03-01",
  school: "",
  guestSchoolName: "",
  allergies: "",
  medication: "",
  healthNotes: "",
  bookings: { "12": "22" },
  days: { "11": ["2027-01-04"] },
  answers: {},
};

describe("holiday day selection", () => {
  it.each([{ days: [] }, { days: ["2027-01-05"] }])(
    "reads only the currently booked school's days: %j",
    ({ days }) => {
      const onChange = vi.fn();
      render(
        <NextIntlClientProvider locale="de" messages={deMessages}>
          <ChildCard
            index={0}
            child={{ ...child, days: { ...child.days, "12": days } }}
            offer={offer}
            removable={false}
            questions={[]}
            questionContext={{ guardianAnswers: {}, fieldsByKey: new Map() }}
            fieldError={() => undefined}
            onChange={onChange}
            onRemove={() => undefined}
          />
        </NextIntlClientProvider>,
      );
      expect(screen.getByRole("button", { name: "Mo 04.01." })).toHaveAttribute(
        "aria-pressed",
        "false",
      );
      const tuesday = screen.getByRole("button", { name: "Di 05.01." });
      expect(tuesday).toHaveAttribute("aria-pressed", String(days.length > 0));
      fireEvent.click(tuesday);
      expect(onChange).toHaveBeenCalledWith(
        expect.objectContaining({
          bookings: { "12": "22" },
          days: {
            "11": ["2027-01-04"],
            "12": days.length ? [] : ["2027-01-05"],
          },
        }),
      );
    },
  );
});
