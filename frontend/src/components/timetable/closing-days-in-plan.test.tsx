import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import type { EnrichedInstance } from "~/lib/timetable-types";

import { MonthPlannerGrid } from "./month-planner-grid";
import { WeeklyCalendarGrid } from "./weekly-calendar-grid";

/**
 * #2032: Schließtage sind im Betreuungsplan (Woche und Monat) erkennbar,
 * inklusive Grund. Ohne die Map bleiben beide Raster verhaltensgleich.
 */

const instance: EnrichedInstance = {
  id: "42",
  date: "2026-05-04",
  startTime: "12:00",
  endTime: "13:00",
  title: "Mensa",
  status: "planned",
  isSpontaneous: false,
  isLive: false,
  activityType: "care",
  roomId: "3",
  roomName: "Mensa",
  staff: [],
  students: [],
  studentIds: [],
  staffCount: 1,
  absentStaffCount: 0,
  expectedStudentsCount: 0,
  notScheduledStudentsCount: 0,
  presentStudentsCount: 0,
  requiredStaffCount: 2,
  assignedStaffCount: 0,
  conflictWarnings: [],
};

const weekDays = [
  new Date("2026-05-04T00:00:00"),
  new Date("2026-05-05T00:00:00"),
  new Date("2026-05-06T00:00:00"),
  new Date("2026-05-07T00:00:00"),
  new Date("2026-05-08T00:00:00"),
];

const CLOSING = new Map([["2026-05-05", "Pädagogischer Tag"]]);

function renderWeek(closingDays?: ReadonlyMap<string, string>) {
  return render(
    <WeeklyCalendarGrid
      weekDays={weekDays}
      instances={[instance]}
      selectedId={null}
      onInstanceClick={vi.fn()}
      todayISO="2026-05-04"
      dayStartHour={9}
      dayEndHour={17}
      hourHeightPx={90}
      closingDays={closingDays}
    />,
  );
}

describe("WeeklyCalendarGrid closing days (#2032)", () => {
  it("labels the closing day column with the reason", () => {
    renderWeek(CLOSING);

    // Tagesheader (Desktop), Tagesstreifen (Mobile) und der Hinweis in der
    // leeren Spalte tragen jeweils den Grund.
    expect(
      screen.getAllByTitle("Schließtag: Pädagogischer Tag").length,
    ).toBeGreaterThan(0);
    expect(
      screen.getAllByText(/Schließtag · Pädagogischer Tag/).length,
    ).toBeGreaterThan(0);
  });

  it("shows nothing extra without closing days", () => {
    renderWeek();

    expect(screen.queryByText(/Schließtag/)).not.toBeInTheDocument();
    expect(screen.getAllByText("Keine Aktivitäten")).toHaveLength(4);
  });
});

describe("MonthPlannerGrid closing days (#2032)", () => {
  const monthDays = [
    new Date("2026-05-04T00:00:00"),
    new Date("2026-05-05T00:00:00"),
    new Date("2026-05-06T00:00:00"),
    new Date("2026-05-07T00:00:00"),
    new Date("2026-05-08T00:00:00"),
    new Date("2026-05-09T00:00:00"),
    new Date("2026-05-10T00:00:00"),
  ];

  it("marks the closing day cell with its reason instead of Leer", () => {
    render(
      <MonthPlannerGrid
        days={monthDays}
        monthDate={new Date("2026-05-01T00:00:00")}
        instances={[]}
        todayISO="2026-05-04"
        closingDays={CLOSING}
        onDayClick={vi.fn()}
      />,
    );

    const closingCell = screen
      .getByTitle("Schließtag: Pädagogischer Tag")
      .closest("button");
    expect(closingCell).not.toBeNull();
    expect(
      within(closingCell as HTMLElement).queryByText("Leer"),
    ).not.toBeInTheDocument();
    // Alle anderen Tage der Woche behalten ihren Leer-Hinweis.
    expect(screen.getAllByText("Leer")).toHaveLength(monthDays.length - 1);
  });

  it("shows nothing extra without closing days", () => {
    render(
      <MonthPlannerGrid
        days={monthDays}
        monthDate={new Date("2026-05-01T00:00:00")}
        instances={[]}
        todayISO="2026-05-04"
        onDayClick={vi.fn()}
      />,
    );

    expect(screen.queryByText(/Schließtag/)).not.toBeInTheDocument();
    expect(screen.getAllByText("Leer")).toHaveLength(monthDays.length);
  });
});
