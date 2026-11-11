import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { HolidayWeekPlaces } from "~/lib/traeger-holiday-registrations-api";
import type { HolidayNoCareDay } from "~/lib/traeger-api";
import { emptyTimeModel, type WeekDraft } from "./holiday-period-model";
import { HolidayWeekEditor } from "./holiday-week-editor";
import { holidayWeekFill } from "./holiday-week-places";

// A statutory holiday in a week by days (#6447): the Träger-Büro sees the
// week with the day struck through and is told that only that day falls
// away; a whole week keeps the warning and names the way out.

const epiphany: HolidayNoCareDay = {
  date: "2027-01-06",
  kind: "holiday",
  name: "Heilige Drei Könige",
};

function week(bookingUnit: "week" | "day"): WeekDraft {
  return {
    key: "w1",
    startsOn: "2027-01-04",
    endsOn: "2027-01-08",
    hostSchoolId: "7",
    timeModels: [emptyTimeModel()],
    bookingUnit,
  };
}

function renderEditor(draft: WeekDraft, days: readonly HolidayNoCareDay[]) {
  render(
    <HolidayWeekEditor
      index={0}
      week={draft}
      schools={[{ id: "7", name: "OGS Nord" } as never]}
      noCareDays={{ days, keptAsStored: false }}
      fieldError={() => undefined}
      onChange={vi.fn()}
      onRemove={vi.fn()}
      onDateValidity={vi.fn()}
    />,
  );
}

describe("holiday week editor with a day without care", () => {
  it("strikes the holiday through and keeps the other days of a week by days", () => {
    renderEditor(week("day"), [epiphany]);
    expect(
      screen.getByText(
        /Eltern können diesen Tag nicht buchen\. Die anderen Tage bleiben buchbar\./,
      ),
    ).toBeInTheDocument();
    const days = screen.getByRole("list", { name: /Buchbare Tage · 4 von 5/ });
    expect(
      within(days).getByLabelText(
        "Mi 06.01.: keine Betreuung, Feiertag (Heilige Drei Könige)",
      ),
    ).toBeInTheDocument();
    expect(
      within(days).getByLabelText("Do 07.01.: buchbar"),
    ).toBeInTheDocument();
  });

  it("warns when no day is left in a week by days", () => {
    const all = ["04", "05", "06", "07", "08"].map((day) => ({
      ...epiphany,
      date: `2027-01-${day}`,
    }));
    renderEditor(week("day"), all);
    expect(
      screen.getByText(/Es bleibt kein Tag zum Buchen\./),
    ).toBeInTheDocument();
  });

  it("points a whole week to single days", () => {
    renderEditor(week("week"), [epiphany]);
    expect(
      screen.getByText(/Wählen Sie bei „Buchung“ „Einzelne Tage“\./),
    ).toBeInTheDocument();
  });
});

describe("holidayWeekFill with a day without care", () => {
  it("counts only the bookable days of a week by days", () => {
    const places: HolidayWeekPlaces = {
      weekId: "51",
      startsOn: "2027-01-04",
      endsOn: "2027-01-08",
      hostSchoolId: "7",
      capacity: 1,
      placed: 0,
      waitlist: [],
      bookingUnit: "day",
      days: [
        { date: "2027-01-04", placed: 1 },
        { date: "2027-01-05", placed: 1 },
        { date: "2027-01-06", placed: 0 },
        { date: "2027-01-07", placed: 1 },
        { date: "2027-01-08", placed: 1 },
      ],
    };
    expect(holidayWeekFill(places, [epiphany])).toEqual({
      label: "Alle Tage voll",
      tone: "warning",
    });
    expect(holidayWeekFill(places).label).toBe("4 Tage voll");
  });
});
