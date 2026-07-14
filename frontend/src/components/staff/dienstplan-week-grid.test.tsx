import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type {
  StaffScheduleAssignment,
  StaffScheduleStaff,
  StaffShift,
  StaffWeeklySummary,
} from "~/lib/shift-helpers";

import { DienstplanWeekGrid } from "./dienstplan-week-grid";

const member: StaffScheduleStaff = {
  id: "7",
  firstName: "Ada",
  lastName: "Lovelace",
};

const shift: StaffShift = {
  id: "9",
  staffId: member.id,
  date: "2026-07-06",
  startTime: "08:00",
  endTime: "12:30",
  breakMinutes: 0,
  shiftTypeId: null,
  shiftTypeName: null,
  shiftTypeColor: null,
  notes: "",
  seriesId: null,
  detached: false,
  cancelled: false,
  changeReason: null,
  originShiftId: null,
};

function assignment(
  overrides: Partial<StaffScheduleAssignment> = {},
): StaffScheduleAssignment {
  return {
    instanceId: "42",
    staffId: member.id,
    date: "2026-07-06",
    startTime: "12:00",
    endTime: "14:00",
    activityTitle: "Mensa",
    roomId: "3",
    roomName: "Speisesaal",
    status: "planned",
    isAbsent: false,
    isSubstitute: false,
    absenceReason: null,
    coverageStatus: "uncovered",
    coverageReason: null,
    uncoveredIntervals: [{ startTime: "12:30", endTime: "14:00" }],
    ...overrides,
  };
}

function renderGrid(
  assignments: StaffScheduleAssignment[],
  summary?: StaffWeeklySummary,
) {
  const onCellClick = vi.fn();
  render(
    <DienstplanWeekGrid
      staff={[member]}
      shiftsByStaff={new Map([[member.id, new Map([[shift.date, [shift]]])]])}
      assignmentsByStaff={
        new Map([[member.id, new Map([[shift.date, assignments]])]])
      }
      summaryByStaff={summary ? new Map([[member.id, summary]]) : new Map()}
      weekDays={[
        "2026-07-06",
        "2026-07-07",
        "2026-07-08",
        "2026-07-09",
        "2026-07-10",
      ]}
      todayIso="2026-07-06"
      typesById={new Map()}
      isLoading={false}
      onCellClick={onCellClick}
    />,
  );
  return onCellClick;
}

describe("DienstplanWeekGrid", () => {
  it("renders a read-only assignment with activity, room and exact gap", () => {
    const onCellClick = renderGrid([assignment()]);

    expect(screen.getByText("Mensa")).toBeInTheDocument();
    expect(screen.getByText("Speisesaal")).toBeInTheDocument();
    expect(screen.getByText("12:00–14:00")).toBeInTheDocument();
    expect(
      screen.getByText("Nicht abgedeckt: 12:30–14:00"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("region", { name: "Dienstplan-Wochenansicht" }),
    ).toHaveAttribute("tabindex", "0");
    expect(
      screen.getByRole("rowheader", { name: "Lovelace, Ada" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Wische horizontal, um weitere Wochentage zu sehen."),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByText("Mensa"));
    expect(onCellClick).not.toHaveBeenCalled();
  });

  it("shows concrete absence and substitute states", () => {
    renderGrid([
      assignment({
        instanceId: "43",
        isAbsent: true,
        isSubstitute: true,
        absenceReason: "krank",
        coverageStatus: "not_applicable",
        coverageReason: "absent",
        uncoveredIntervals: [],
      }),
      assignment({
        instanceId: "44",
        activityTitle: "Lernzeit",
        isSubstitute: true,
        coverageStatus: "covered",
        uncoveredIntervals: [],
      }),
    ]);

    expect(screen.getByText("Abwesend · krank")).toBeInTheDocument();
    expect(screen.getByText("Vertretung")).toBeInTheDocument();
    expect(screen.queryByText(/Nicht abgedeckt:/)).not.toBeInTheDocument();
  });

  it("keeps shift editing separate from assignment cards", () => {
    const onCellClick = renderGrid([assignment()]);

    fireEvent.click(screen.getByRole("button", { name: "08:00–12:30" }));
    expect(onCellClick).toHaveBeenCalledWith(member, shift.date, shift);
  });

  it("shows planned hours and the delta against the contract", () => {
    renderGrid([], {
      staffId: member.id,
      weekStart: "2026-07-06",
      plannedMinutes: 1080,
      targetMinutes: 1215,
      deltaMinutes: -135,
    });

    expect(screen.getByText("18 h")).toBeInTheDocument();
    const delta = screen.getByText("· −2,25 h");
    // Under contract renders in HOME red.
    expect(delta).toHaveStyle({ color: "#FF3130" });

    // The Geplant/Soll breakdown is a real tooltip (not a title attribute),
    // reachable via keyboard focus and touch.
    expect(screen.getByRole("tooltip")).toHaveTextContent(
      "Geplant 18 h · Soll 20,25 h",
    );
    expect(screen.getByText("18 h").closest("[tabindex]")).toHaveAttribute(
      "tabindex",
      "0",
    );
  });

  it("shows planned hours without a badge when no contract resolves", () => {
    renderGrid([], {
      staffId: member.id,
      weekStart: "2026-07-06",
      plannedMinutes: 270,
      targetMinutes: null,
      deltaMinutes: null,
    });

    expect(screen.getByText("4,5 h")).toBeInTheDocument();
    // No delta badge (the tooltip text legitimately contains "·" now).
    expect(screen.queryByText(/^·/)).not.toBeInTheDocument();
    expect(screen.getByRole("tooltip")).toHaveTextContent(
      "Geplant 4,5 h · kein Arbeitszeitmodell hinterlegt",
    );
  });

  it("renders only the name without a summary", () => {
    renderGrid([]);

    expect(
      screen.getByRole("rowheader", { name: "Lovelace, Ada" }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/ h$/)).not.toBeInTheDocument();
  });
});

describe("DienstplanWeekGrid series indicator", () => {
  function renderGridWithShift(overrides: Partial<StaffShift>) {
    const seriesShift = { ...shift, ...overrides };
    render(
      <DienstplanWeekGrid
        staff={[member]}
        shiftsByStaff={
          new Map([[member.id, new Map([[shift.date, [seriesShift]]])]])
        }
        assignmentsByStaff={new Map()}
        summaryByStaff={new Map()}
        weekDays={[shift.date]}
        todayIso={shift.date}
        typesById={new Map()}
        isLoading={false}
        onCellClick={vi.fn()}
      />,
    );
  }

  it("shows no series icon on a standalone shift", () => {
    renderGridWithShift({ seriesId: null });

    expect(screen.queryByLabelText("Teil einer Serie")).not.toBeInTheDocument();
    expect(
      screen.queryByLabelText("Serie, für diese Woche angepasst"),
    ).not.toBeInTheDocument();
  });

  it("shows a neutral series icon on a series-backed shift", () => {
    renderGridWithShift({ seriesId: "5" });

    const icon = screen.getByLabelText("Teil einer Serie");
    expect(icon).toHaveClass("text-gray-400");
  });

  it("shows the detached icon in the brand amber on a deviated shift", () => {
    renderGridWithShift({ seriesId: "5", detached: true });

    const icon = screen.getByLabelText("Serie, für diese Woche angepasst");
    expect(icon).toHaveClass("text-[#EAB308]");
  });
});

describe("DienstplanWeekGrid flexible changes", () => {
  function renderGridWithShift(overrides: Partial<StaffShift>) {
    const flexShift = { ...shift, ...overrides };
    render(
      <DienstplanWeekGrid
        staff={[member]}
        shiftsByStaff={
          new Map([[member.id, new Map([[shift.date, [flexShift]]])]])
        }
        assignmentsByStaff={new Map()}
        summaryByStaff={new Map()}
        weekDays={[shift.date]}
        todayIso={shift.date}
        typesById={new Map()}
        isLoading={false}
        onCellClick={vi.fn()}
      />,
    );
  }

  it("marks a cancelled shift as 'Fällt aus' with its reason (#1841)", () => {
    renderGridWithShift({ cancelled: true, changeReason: "Krankheit" });

    expect(screen.getByText("Fällt aus · Krankheit")).toBeInTheDocument();
  });

  it("labels a replacement shift as 'Vertretung' (#1841)", () => {
    renderGridWithShift({ originShiftId: "9", changeReason: "Tausch" });

    expect(screen.getByText("Vertretung · Tausch")).toBeInTheDocument();
  });
});

describe("DienstplanWeekGrid sick report (#1843)", () => {
  function renderWithSick(onSickReport?: (staff: StaffScheduleStaff) => void) {
    render(
      <DienstplanWeekGrid
        staff={[member]}
        shiftsByStaff={new Map()}
        assignmentsByStaff={new Map()}
        summaryByStaff={new Map()}
        weekDays={["2026-07-06"]}
        todayIso="2026-07-06"
        typesById={new Map()}
        isLoading={false}
        onCellClick={vi.fn()}
        onSickReport={onSickReport}
      />,
    );
  }

  it("renders no sick-report action without the callback", () => {
    renderWithSick();

    expect(
      screen.queryByRole("button", { name: "Krank melden für Ada Lovelace" }),
    ).not.toBeInTheDocument();
  });

  it("fires onSickReport with the row's member", () => {
    const onSickReport = vi.fn();
    renderWithSick(onSickReport);

    fireEvent.click(
      screen.getByRole("button", { name: "Krank melden für Ada Lovelace" }),
    );
    expect(onSickReport).toHaveBeenCalledWith(member);
  });
});
