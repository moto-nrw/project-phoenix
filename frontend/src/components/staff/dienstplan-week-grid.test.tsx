import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type {
  StaffScheduleAssignment,
  StaffScheduleStaff,
  StaffShift,
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
  notes: "",
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

function renderGrid(assignments: StaffScheduleAssignment[]) {
  const onCellClick = vi.fn();
  render(
    <DienstplanWeekGrid
      staff={[member]}
      shiftsByStaff={new Map([[member.id, new Map([[shift.date, [shift]]])]])}
      assignmentsByStaff={
        new Map([[member.id, new Map([[shift.date, assignments]])]])
      }
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
});
