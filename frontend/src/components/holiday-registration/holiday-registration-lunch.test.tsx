// Mittagessen in der Ferienanmeldung der Eltern (#5631): die Wahl je Kind
// und Woche, was sie kostet und was die Anmeldung schickt.

import { fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.generated";
import type { HolidayRegistrationOffer } from "~/lib/holiday-registration-api";
import { HOLIDAY_CONSENT_TEXTS } from "~/test/fixtures/holiday-consent-texts";
import {
  bookedWeeks,
  type ChildDraft,
  quoteBookings,
  weekLunch,
} from "./holiday-registration-model";
import { ChildCard } from "./holiday-registration-sections";

const timeModel = (id: string, priceCents: number) => ({
  id,
  name: "bis 16 Uhr",
  startsAt: "07:45",
  endsAt: "16:00",
  priceCents,
});

const offer: HolidayRegistrationOffer = {
  traegerName: "Träger Beispiel",
  holidayWishMode: "off",
  name: "Sommerferien",
  registrationOpensOn: "2027-04-01",
  registrationClosesOn: "2027-05-31",
  binding: true,
  notes: "",
  open: true,
  captchaSiteKey: "site",
  consentTexts: HOLIDAY_CONSENT_TEXTS,
  form: { version: 1, fields: [] },
  maxDaysPerChild: null,
  weeks: [
    {
      id: "11",
      startsOn: "2027-07-12",
      endsOn: "2027-07-16",
      hostSchoolName: "OGS Nord",
      timeModels: [timeModel("21", 6500)],
      lunchPriceCents: 430,
      lunchDays: 5,
    },
    {
      id: "12",
      startsOn: "2027-07-19",
      endsOn: "2027-07-23",
      hostSchoolName: "OGS Nord",
      bookingUnit: "day",
      careDays: ["2027-07-19", "2027-07-20", "2027-07-21"],
      timeModels: [timeModel("22", 1300)],
      lunchPriceCents: 430,
      lunchDays: 5,
    },
    {
      id: "13",
      startsOn: "2027-07-26",
      endsOn: "2027-07-30",
      hostSchoolName: "OGS Nord",
      timeModels: [timeModel("23", 6500)],
    },
  ],
  schools: [],
};

const child: ChildDraft = {
  key: "child",
  firstName: "Mia",
  lastName: "Muster",
  birthDate: "2018-03-01",
  school: "",
  guestSchoolName: "",
  allergies: "",
  medication: "",
  healthNotes: "",
  bookings: { "11": "21", "13": "23" },
  answers: {},
};

function renderCard(draft: ChildDraft, onChange = vi.fn()) {
  render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ChildCard
        index={0}
        child={draft}
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
  return onChange;
}

describe("lunch in the holiday registration", () => {
  it("offers lunch only for a booked week that has it, with what it costs", () => {
    const onChange = renderCard(child);
    // Week 11 is booked and offers lunch; week 13 is booked without lunch;
    // week 12 offers lunch but is not booked.
    const boxes = screen.getAllByRole("checkbox", { name: /Mit Mittagessen/ });
    expect(boxes).toHaveLength(1);
    expect(boxes[0]).not.toBeChecked();
    expect(
      // The test catalog leaves the plural form as written; the sum is in it.
      screen.getByText(/Essen zu je 4,30\s€, zusammen 21,50\s€/),
    ).toBeInTheDocument();
    fireEvent.click(boxes[0]!);
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ lunch: { "11": true } }),
    );
  });

  it("names the price of one lunch until a day is chosen in a week by days", () => {
    renderCard({ ...child, bookings: { "12": "22" } });
    expect(
      screen.getByText(
        /^4,30\s€ je Essen\. Ihr Kind isst an jedem Tag, den Sie wählen\.$/,
      ),
    ).toBeInTheDocument();
  });

  it("counts one lunch per chosen day and the lunches of a whole week", () => {
    const [whole, byDays, without] = offer.weeks;
    expect(weekLunch(whole!, [])).toEqual({ lunches: 5, cents: 2150 });
    expect(weekLunch(byDays!, ["2027-07-19", "2027-07-21"])).toEqual({
      lunches: 2,
      cents: 860,
    });
    // A day the week does not offer counts no lunch.
    expect(weekLunch(byDays!, ["2027-07-23"])).toEqual({
      lunches: 0,
      cents: 0,
    });
    expect(weekLunch(without!, [])).toBeNull();
  });

  it("sends lunch only for booked weeks that offer it", () => {
    const draft: ChildDraft = {
      ...child,
      // Ticked once for week 12, which is no longer booked, and for week 13,
      // which offers no lunch: neither is sent.
      lunch: { "11": true, "12": true, "13": true },
    };
    expect(bookedWeeks(draft, offer).map((booking) => booking.lunch)).toEqual([
      true,
      false,
    ]);
    expect(quoteBookings(draft, offer)).toEqual([
      { week_id: 11, time_model_id: 21, lunch: true },
      { week_id: 13, time_model_id: 23 },
    ]);
  });
});
