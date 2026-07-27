import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import { WeeklyCalendarGrid } from "./weekly-calendar-grid";
import type { EnrichedInstance } from "~/lib/timetable-types";

/**
 * Guards the optional `gapInstanceIds` prop (Betreuungsplan Abschnitt 5.1/5.2):
 * a block whose instance id is in the set renders the single gap status icon;
 * without the prop no block shows one, so both existing call-sites stay
 * behavior-equal.
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

function renderGrid(gapInstanceIds?: ReadonlySet<string>) {
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
      gapInstanceIds={gapInstanceIds}
    />,
  );
}

describe("WeeklyCalendarGrid gapInstanceIds", () => {
  it("marks a block whose id is in the gap set with the gap icon", () => {
    renderGrid(new Set(["42"]));

    expect(screen.getByLabelText("Offene Lücke")).toBeInTheDocument();
  });

  it("shows no gap icon when the block id is not in the set", () => {
    renderGrid(new Set(["999"]));

    expect(screen.queryByLabelText("Offene Lücke")).not.toBeInTheDocument();
  });

  it("shows no gap icon without the prop (default behavior)", () => {
    renderGrid();

    expect(screen.queryByLabelText("Offene Lücke")).not.toBeInTheDocument();
  });
});

/**
 * Guards the optional `showDayHeader` prop (06-betreuungsplan.md Abschnitt
 * 3.1): opt-in, default off — the Vertretung single-day usage of this grid
 * never sets it and keeps rendering without a day-header capacity row.
 */
describe("WeeklyCalendarGrid showDayHeader", () => {
  const staffedInstance: EnrichedInstance = {
    ...instance,
    id: "staffed",
    staff: [
      { staffId: "s1", isPrimary: true, isAbsent: false, isSubstitute: false },
    ],
  };

  it("renders no day-header capacity row without the prop (Vertretung regression)", () => {
    render(
      <WeeklyCalendarGrid
        weekDays={weekDays}
        instances={[staffedInstance]}
        selectedId={null}
        onInstanceClick={vi.fn()}
        todayISO="2026-05-04"
        dayStartHour={9}
        dayEndHour={17}
        hourHeightPx={90}
      />,
    );

    expect(screen.queryByText("1 P.")).not.toBeInTheDocument();
  });

  it("shows the planned person count per day, only 'N P.' without a child value", () => {
    const { container } = render(
      <WeeklyCalendarGrid
        weekDays={weekDays}
        instances={[staffedInstance]}
        selectedId={null}
        onInstanceClick={vi.fn()}
        todayISO="2026-05-04"
        dayStartHour={9}
        dayEndHour={17}
        hourHeightPx={90}
        showDayHeader
      />,
    );

    // The staffed day shows "1 P.", the four empty days show "0 P." — no
    // child value (no "~") is rendered anywhere (Kriterium 10 Übergang).
    expect(screen.getByText("1 P.")).toBeInTheDocument();
    expect(screen.getAllByText("0 P.")).toHaveLength(4);
    expect(screen.queryByText(/~/)).not.toBeInTheDocument();
    // Die Kapazitätszeile liegt im selben Scroll-Container wie der Header.
    // Ihr eigenes Höhenbudget bewahrt die vorher sichtbaren 720px Raster.
    expect(
      container.querySelector(".sm\\:max-h-\\[808px\\]"),
    ).toBeInTheDocument();
  });
});
