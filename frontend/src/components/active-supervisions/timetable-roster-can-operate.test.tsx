import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TimetableRosterContent } from "./timetable-roster";
import type {
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";

const VIEW_ONLY_HINT = "Sie sind für diese Aktivität nicht eingeplant.";

function rosterRow(overrides: Partial<TimetableRosterRow>): TimetableRosterRow {
  return {
    studentId: "1",
    studentName: "Marie Muster",
    schoolClass: "2b",
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
    pickupTime: null,
    warnings: [],
    careDayStatus: "scheduled",
    parallelPresentIn: null,
    ...overrides,
  };
}

function roster(canOperate: boolean | undefined): TimetableRoster {
  return {
    instance: {
      id: "instance-1",
      title: "Randstunde",
      status: "active",
      isSpontaneous: false,
      activeGroupId: "group-1",
      roomId: "room-1",
      roomName: "Raum 1",
      date: "2026-09-09",
      startTime: "11:45",
      endTime: "13:30",
      canComplete: false,
      completeAvailableAt: "2026-09-09T11:30:00Z",
    },
    rows: [
      rosterRow({}),
      rosterRow({
        studentId: "2",
        studentName: "Ben Beispiel",
        currentlyPresent: true,
        status: "present",
        visitId: "20",
      }),
    ],
    pickupTimesLoaded: true,
    ...(canOperate === undefined ? {} : { canOperate }),
  };
}

function renderRoster(value: TimetableRoster, attendanceWebEnabled = true) {
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
      onAddStudent={vi.fn()}
      onComplete={vi.fn()}
      onConfirmExpected={vi.fn()}
      onRosterAction={vi.fn()}
      onSearchChange={vi.fn()}
    />,
  );
}

describe("TimetableRosterContent action rights (#3167)", () => {
  it("allows absence reports and their reversal without block lifecycle rights", () => {
    const value = {
      ...roster(false),
      canEditAttendance: false,
      canReportAbsence: true,
    };
    value.rows.push(
      rosterRow({ studentId: "3", status: "absent", substatus: "excused" }),
    );
    renderRoster(value);
    expect(
      screen.getByRole("button", { name: "Entschuldigt" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Zurück auf erwartet" }),
    ).toBeInTheDocument();
    for (const name of ["Einchecken", "Abwesend", /^Beenden/]) {
      expect(screen.queryByRole("button", { name })).not.toBeInTheDocument();
    }
    expect(
      screen.queryByText(new RegExp(VIEW_ONLY_HINT)),
    ).not.toBeInTheDocument();
  });
  it("allows school-wide check-in and check-out without lifecycle or status actions", () => {
    renderRoster({
      ...roster(false),
      canEditAttendance: true,
      canReportAbsence: false,
    });
    for (const name of [
      "Einchecken",
      "Raum verlassen",
      "Kind hinzufügen",
      "Erwartete bestätigen",
    ]) {
      expect(screen.getByRole("button", { name })).toBeInTheDocument();
    }
    for (const name of ["Entschuldigt", "Abwesend", /^Beenden/]) {
      expect(screen.queryByRole("button", { name })).not.toBeInTheDocument();
    }
    expect(
      screen.queryByText(new RegExp(VIEW_ONLY_HINT)),
    ).not.toBeInTheDocument();
  });

  it("keeps other block actions when sick and excused reports are restricted", () => {
    const value = {
      ...roster(true),
      canEditAttendance: true,
      canReportAbsence: false,
    };
    value.rows.push(
      rosterRow({ studentId: "3", status: "absent", substatus: "excused" }),
    );
    renderRoster(value);
    expect(
      screen.queryByRole("button", { name: "Entschuldigt" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Zurück auf erwartet" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Einchecken" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Abwesend" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /^Beenden/ }),
    ).toBeInTheDocument();
  });

  it("does not expose web attendance actions through a school-wide grant when web is off", () => {
    renderRoster({ ...roster(false), canEditAttendance: true }, false);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("shows the list without any action to a caller who is not planned", () => {
    renderRoster(roster(false));

    expect(screen.getByText("Marie Muster")).toBeInTheDocument();
    expect(screen.getByText("Ben Beispiel")).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.getByText(new RegExp(VIEW_ONLY_HINT))).toBeInTheDocument();
  });

  it.each([true, undefined])(
    "keeps every action when canOperate is %s",
    (canOperate) => {
      renderRoster(roster(canOperate));

      for (const name of [
        "Einchecken",
        "Raum verlassen",
        "Entschuldigt",
        "Abwesend",
        "Kind hinzufügen",
        "Erwartete bestätigen",
        /^Beenden/,
      ]) {
        expect(screen.getByRole("button", { name })).toBeInTheDocument();
      }
      expect(
        screen.queryByText(new RegExp(VIEW_ONLY_HINT)),
      ).not.toBeInTheDocument();
    },
  );
});
