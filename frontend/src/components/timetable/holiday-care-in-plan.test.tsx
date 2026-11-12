import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import type { HolidayCareDay } from "~/lib/holiday-care";

import { MonthPlannerGrid } from "./month-planner-grid";
import { WeeklyCalendarGrid } from "./weekly-calendar-grid";

/**
 * Ferienbetreuung für den Träger (ADR 0116) im Betreuungsplan: Woche, Tag und
 * Monat nennen sie mit Kindern und Betreuungszeit. Ein Schließtag schlägt sie,
 * weil die Schule an ihm keine Ferienbetreuung gibt.
 */

const weekDays = [
  new Date("2027-01-04T00:00:00"),
  new Date("2027-01-05T00:00:00"),
  new Date("2027-01-06T00:00:00"),
  new Date("2027-01-07T00:00:00"),
  new Date("2027-01-08T00:00:00"),
];

const CARE: HolidayCareDay = {
  weekId: "3",
  children: 3,
  startsAt: "07:45",
  endsAt: "16:00",
};

const HOLIDAY_CARE = new Map<string, HolidayCareDay>(
  ["2027-01-04", "2027-01-05", "2027-01-07", "2027-01-08"].map((iso) => [
    iso,
    CARE,
  ]),
);
const CLOSING = new Map([["2027-01-06", "Pädagogischer Tag"]]);
const SENTENCE =
  "Ferienbetreuung für den Träger: 3 Kinder, 07:45 bis 16:00 Uhr.";

function renderWeek(
  props: Partial<React.ComponentProps<typeof WeeklyCalendarGrid>> = {},
) {
  return render(
    <WeeklyCalendarGrid
      weekDays={weekDays}
      instances={[]}
      selectedId={null}
      onInstanceClick={vi.fn()}
      todayISO="2027-01-04"
      dayStartHour={9}
      dayEndHour={17}
      hourHeightPx={90}
      closingDays={CLOSING}
      holidayCare={HOLIDAY_CARE}
      {...props}
    />,
  );
}

describe("WeeklyCalendarGrid holiday care", () => {
  it("names the holiday care in the day heads and shows its care window in the columns", () => {
    const { container } = renderWeek();

    // Tageskopf (Desktop): „Ferienbetreuung · 3 Kinder“, die Zeit trägt das
    // Band in der Spalte.
    expect(
      screen.getAllByText("Ferienbetreuung · 3 Kinder").length,
    ).toBeGreaterThanOrEqual(4);
    expect(screen.getAllByTitle(SENTENCE).length).toBeGreaterThanOrEqual(4);
    const bands = container.querySelectorAll("[data-holiday-care-band]");
    expect(bands).toHaveLength(4);
    expect(bands[0]).toHaveTextContent("07:45–16:00");
    // Die Betreuung beginnt vor dem Tagesbeginn des Rasters: es wächst bis
    // 07:00 nach oben, wie für einen frühen Termin.
    expect(screen.getByText("07:00")).toBeInTheDocument();
  });

  it("shows the closing day, not the holiday care, on a closing day of the week", () => {
    renderWeek();

    expect(
      screen.getAllByText(/Schließtag · Pädagogischer Tag/).length,
    ).toBeGreaterThan(0);
    const wednesday = screen.getByLabelText("Mi 06.01.");
    const head = wednesday.closest("div.flex-col");
    expect(head).not.toBeNull();
    expect(
      within(head as HTMLElement).getByText(/Schließtag/),
    ).toBeInTheDocument();
    expect(
      within(head as HTMLElement).queryByText(/Ferienbetreuung/),
    ).not.toBeInTheDocument();
  });

  it("links the marker to the holiday weeks when the person may open them", () => {
    renderWeek({ holidayCareHref: "/ferienwochen" });

    const links = screen.getAllByRole("link", {
      name: "Ferienbetreuung · 3 Kinder · 07:45–16:00",
    });
    expect(links).toHaveLength(4);
    expect(links[0]).toHaveAttribute("href", "/ferienwochen");
  });

  it("is a plain display without the right to open the holiday weeks", () => {
    renderWeek();

    expect(screen.getAllByTitle(SENTENCE).length).toBeGreaterThan(0);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("stays unchanged without holiday care", () => {
    const { container } = renderWeek({ holidayCare: undefined });

    // Anker: der Schließtag steht, das Raster beginnt beim Tagesbeginn.
    expect(
      screen.getAllByText(/Schließtag · Pädagogischer Tag/).length,
    ).toBeGreaterThan(0);
    expect(screen.getByText("09:00")).toBeInTheDocument();
    expect(screen.queryByText(/Ferienbetreuung/)).not.toBeInTheDocument();
    expect(container.querySelector("[data-holiday-care-band]")).toBeNull();
  });
});

describe("MonthPlannerGrid holiday care", () => {
  const monthDays = [
    ...weekDays,
    new Date("2027-01-09T00:00:00"),
    new Date("2027-01-10T00:00:00"),
  ];

  it("names the holiday care with children and care window instead of Leer", () => {
    render(
      <MonthPlannerGrid
        days={monthDays}
        monthDate={new Date("2027-01-01T00:00:00")}
        instances={[]}
        todayISO="2027-01-04"
        closingDays={CLOSING}
        holidayCare={HOLIDAY_CARE}
        holidayCareHref="/ferienwochen"
        onDayClick={vi.fn()}
        onInstanceClick={vi.fn()}
      />,
    );

    expect(screen.getAllByText("3 Kinder · 07:45–16:00")).toHaveLength(4);
    const monday = screen.getByRole("button", { name: "Mo 4.1.2027" })
      .parentElement as HTMLElement;
    expect(
      within(monday).getAllByRole("link", {
        name: "Ferienbetreuung · 3 Kinder · 07:45–16:00",
      })[0],
    ).toHaveAttribute("href", "/ferienwochen");
    expect(within(monday).queryByText("Leer")).not.toBeInTheDocument();
    // Der Schließtag und das Wochenende behalten ihre bisherige Anzeige.
    expect(
      screen.getByTitle("Schließtag: Pädagogischer Tag"),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Leer")).toHaveLength(2);
  });
});
