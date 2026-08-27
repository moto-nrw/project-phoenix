import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TimetableRosterContent } from "./timetable-roster";
import type {
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";

function rosterRow(
  studentId: string,
  studentName: string,
  pickupTime: string | null,
  overrides: Partial<TimetableRosterRow> = {},
): TimetableRosterRow {
  return {
    studentId,
    studentName,
    schoolClass: "2a",
    groupName: "Sonnengruppe",
    planned: true,
    isUnplanned: false,
    currentlyPresent: false,
    visitId: null,
    status: "expected",
    substatus: null,
    note: null,
    checkedInAt: null,
    checkedOutAt: null,
    visitEntryTime: null,
    pickupTime,
    warnings: [],
    careDayStatus: "scheduled",
    parallelPresentIn: null,
    ...overrides,
  };
}

function roster(
  rows: TimetableRosterRow[],
  pickupTimesLoaded: boolean,
): TimetableRoster {
  return {
    instance: {
      id: "instance-1",
      title: "Randstunde",
      status: "active",
      isSpontaneous: false,
      activeGroupId: "group-1",
      roomId: "room-1",
      roomName: "Raum 1",
      date: "2026-08-31",
      startTime: "12:45",
      endTime: "13:30",
      canComplete: false,
      completeAvailableAt: "2026-08-31T13:30:00Z",
    },
    rows,
    pickupTimesLoaded,
  };
}

function renderRoster(value: TimetableRoster, attendanceWebEnabled = false) {
  render(
    <TimetableRosterContent
      addStudentResults={[]}
      addStudentSearch=""
      attendanceWebEnabled={attendanceWebEnabled}
      isAddingStudent={false}
      isCompletingInstance={false}
      isConfirmingExpected={false}
      roster={value}
      showTimetableCounts={false}
      canAddUnplanned={false}
      onAddStudent={vi.fn()}
      onComplete={vi.fn()}
      onConfirmExpected={vi.fn()}
      onRosterAction={vi.fn()}
      onSearchChange={vi.fn()}
    />,
  );
}

describe("TimetableRosterContent pickup times", () => {
  it("shows a pickup time on every roster section and a neutral placeholder when none exists", () => {
    renderRoster(
      roster(
        [
          rosterRow("1", "Anwesend Kind", "13:00", {
            currentlyPresent: true,
            status: "present",
          }),
          rosterRow("2", "Erwartet Kind", "13:10"),
          rosterRow("3", "Nicht eingeplant Kind", "13:20", {
            careDayStatus: "not_scheduled",
          }),
          rosterRow("4", "Abwesend Kind", "13:30", { status: "absent" }),
          rosterRow("5", "Gegangen Kind", "13:40", { status: "present" }),
          rosterRow("6", "Ungeplant Kind", null, {
            planned: false,
            isUnplanned: true,
            currentlyPresent: true,
            status: "present",
          }),
        ],
        true,
      ),
    );

    for (const time of ["13:00", "13:10", "13:20", "13:30", "13:40"]) {
      expect(screen.getByText(`Gehzeit: ${time}`)).toBeInTheDocument();
    }
    expect(screen.getByText("Gehzeit: —")).toBeInTheDocument();
  });

  it("shows a load error without replacing it with the empty-time placeholder", () => {
    renderRoster(roster([rosterRow("7", "Weiter nutzbar", null)], false), true);

    expect(
      screen.getByText(
        "Die Gehzeiten konnten nicht geladen werden. Die Anwesenheitsliste bleibt verfügbar.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Gehzeit: Nicht geladen")).toBeInTheDocument();
    expect(screen.queryByText("Gehzeit: —")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Einchecken" }),
    ).toBeInTheDocument();
  });
});
