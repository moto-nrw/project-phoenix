import { fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.generated";
import type { HolidayRegistrationOffer } from "~/lib/holiday-registration-api";
import { HOLIDAY_CONSENT_TEXTS } from "~/test/fixtures/holiday-consent-texts";
import type { ChildDraft } from "./holiday-registration-model";
import { ChildCard } from "./holiday-registration-sections";

// A week by days with a statutory holiday (#6447): the holiday stays in the
// row of days, struck through and not selectable; the other days are.
const offer: HolidayRegistrationOffer = {
  traegerName: "Träger Beispiel",
  holidayWishMode: "off",
  name: "Winterferien",
  registrationOpensOn: "2026-09-01",
  registrationClosesOn: "2026-12-04",
  binding: true,
  notes: "",
  open: true,
  captchaSiteKey: "site",
  consentTexts: HOLIDAY_CONSENT_TEXTS,
  form: { version: 1, fields: [] },
  maxDaysPerChild: 3,
  weeks: [
    {
      id: "11",
      startsOn: "2027-01-04",
      endsOn: "2027-01-08",
      bookingUnit: "day",
      careDays: ["2027-01-04", "2027-01-05", "2027-01-07", "2027-01-08"],
      noCareDays: [{ date: "2027-01-06", name: "Heilige Drei Könige" }],
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
  bookings: { "11": "21" },
  days: { "11": ["2027-01-04"] },
  answers: {},
};

function renderChild(onChange = vi.fn()) {
  render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ChildCard
        index={0}
        child={child}
        offer={offer}
        removable={false}
        questions={[]}
        questionContext={{ guardianAnswers: {}, fieldsByKey: new Map() }}
        fieldError={() => undefined}
        onChange={onChange}
        onRemove={vi.fn()}
      />
    </NextIntlClientProvider>,
  );
  return onChange;
}

describe("holiday day without care in a week by days (#6447)", () => {
  it("shows the holiday struck through and keeps it from being chosen", () => {
    const onChange = renderChild();
    const holiday = screen.getByRole("button", {
      name: "Mi 06.01., keine Betreuung",
    });
    expect(holiday).toBeDisabled();
    fireEvent.click(holiday);
    expect(onChange).not.toHaveBeenCalled();
    expect(
      screen.getByText(/Am Mi 06\.01\. ist keine Betreuung/),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Do 07.01." }));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        days: { "11": ["2027-01-04", "2027-01-07"] },
      }),
    );
  });

  it("says that the bound counts all registrations of the child (#6446)", () => {
    renderChild();
    expect(
      screen.getByText(/Das gilt für alle Anmeldungen des Kindes zusammen\./),
    ).toBeInTheDocument();
  });
});
