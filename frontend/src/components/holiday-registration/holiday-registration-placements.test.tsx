import { render, screen, within } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it } from "vitest";
import deMessages from "~/i18n/messages/de.generated";
import type { HolidayRegistrationWeek } from "~/lib/holiday-registration-api";
import { HolidayRegistrationPlacements } from "./holiday-registration-placements";
import type { ChildDraft } from "./holiday-registration-model";

const WEEKS: HolidayRegistrationWeek[] = [
  {
    id: "51",
    startsOn: "2027-07-05",
    endsOn: "2027-07-09",
    hostSchoolName: "OGS am Berg",
    timeModels: [],
  },
  {
    id: "52",
    startsOn: "2027-07-12",
    endsOn: "2027-07-16",
    hostSchoolName: "OGS am Berg",
    timeModels: [],
  },
];

function child(key: string, firstName: string): ChildDraft {
  return {
    key,
    firstName,
    lastName: "Muster",
    birthDate: "2018-03-01",
    school: "31",
    guestSchoolName: "",
    allergies: "",
    medication: "",
    healthNotes: "",
    bookings: {},
    answers: {},
  };
}

function renderPlacements(
  placements: Parameters<typeof HolidayRegistrationPlacements>[0]["placements"],
) {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <HolidayRegistrationPlacements
        childDrafts={[child("a", "Lia"), child("b", "Ben")]}
        weeks={WEEKS}
        placements={placements}
      />
    </NextIntlClientProvider>,
  );
}

describe("HolidayRegistrationPlacements", () => {
  it("names a place or the waiting list for each child and week, siblings apart", () => {
    renderPlacements([
      { childPosition: 1, weekId: "51", placement: "placed" },
      { childPosition: 2, weekId: "51", placement: "waitlisted" },
      { childPosition: 2, weekId: "52", placement: "placed" },
    ]);
    const lia = screen.getByText("Lia Muster").closest("li");
    const ben = screen.getByText("Ben Muster").closest("li");
    expect(lia).not.toBeNull();
    expect(ben).not.toBeNull();
    expect(
      within(lia as HTMLElement).getAllByText("Platz reserviert"),
    ).toHaveLength(1);
    expect(
      within(ben as HTMLElement).getByText("Warteliste"),
    ).toBeInTheDocument();
    expect(
      within(ben as HTMLElement).getAllByText("Platz reserviert"),
    ).toHaveLength(1);
    expect(
      screen.getByText(/Wird ein Platz frei, bekommt Ihr Kind ihn automatisch/),
    ).toBeInTheDocument();
  });

  it("explains no waiting list when every week has a place", () => {
    renderPlacements([{ childPosition: 1, weekId: "51", placement: "placed" }]);
    expect(screen.getByText("Platz reserviert")).toBeInTheDocument();
    expect(screen.queryByText(/Wird ein Platz frei/)).toBeNull();
  });

  it("names an open place when the office gives the places (#4700)", () => {
    renderPlacements([
      { childPosition: 1, weekId: "51", placement: "pending" },
      { childPosition: 2, weekId: "51", placement: "pending" },
    ]);
    expect(screen.getAllByText("Platz offen")).toHaveLength(2);
    expect(
      screen.getByText(
        "Das Büro vergibt die Plätze. Sie bekommen eine E-Mail.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Platz reserviert")).not.toBeInTheDocument();
  });
});
