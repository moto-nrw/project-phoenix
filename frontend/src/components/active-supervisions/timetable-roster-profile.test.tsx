import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TimetableRosterContent } from "./timetable-roster";
import type {
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";

function rosterRow(
  studentId: string,
  studentName: string,
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
    pickupTime: null,
    warnings: [],
    careDayStatus: "scheduled",
    parallelPresentIn: null,
    ...overrides,
  };
}

const ROSTER: TimetableRoster = {
  instance: {
    id: "instance-1",
    title: "Randstunde",
    status: "active",
    isSpontaneous: false,
    activeGroupId: "group-1",
    roomId: "room-1",
    roomName: "Raum 1",
    date: "2026-09-09",
    startTime: "11:00",
    endTime: "13:30",
    canComplete: false,
    completeAvailableAt: "2026-09-09T13:30:00Z",
  },
  rows: [
    rosterRow("1", "Anna Anwesend", {
      currentlyPresent: true,
      status: "present",
    }),
    rosterRow("2", "Ben Erwartet"),
  ],
  pickupTimesLoaded: true,
};

function renderRoster({
  attendanceWebEnabled = true,
  onRosterAction = vi.fn(),
  studentProfileHref,
}: {
  attendanceWebEnabled?: boolean;
  onRosterAction?: () => Promise<void>;
  studentProfileHref?: (row: TimetableRosterRow) => string;
}) {
  render(
    <TimetableRosterContent
      addStudentSearch=""
      attendanceWebEnabled={attendanceWebEnabled}
      isAddingStudent={false}
      isCompletingInstance={false}
      isConfirmingExpected={false}
      roster={ROSTER}
      showTimetableCounts={false}
      canAddUnplanned={false}
      onAddStudent={vi.fn()}
      onComplete={vi.fn()}
      onConfirmExpected={vi.fn()}
      onRosterAction={onRosterAction}
      onSearchChange={vi.fn()}
      studentProfileHref={studentProfileHref}
    />,
  );
}

const profileHref = (row: TimetableRosterRow) =>
  `/students/${row.studentId}?from=/active-supervisions`;

describe("TimetableRosterContent „Profil ansehen“ (#4356)", () => {
  it("links every row to the child's profile, next to the attendance action", () => {
    renderRoster({ studentProfileHref: profileHref });

    expect(
      screen.getByRole("link", { name: "Profil ansehen: Anna Anwesend" }),
    ).toHaveAttribute("href", "/students/1?from=/active-supervisions");
    expect(
      screen.getByRole("link", { name: "Profil ansehen: Ben Erwartet" }),
    ).toHaveAttribute("href", "/students/2?from=/active-supervisions");
    expect(
      screen.getByRole("button", { name: "Raum verlassen" }),
    ).toBeInTheDocument();
  });

  it("does not change attendance when opened", () => {
    const onRosterAction = vi.fn(async () => undefined);
    renderRoster({ studentProfileHref: profileHref, onRosterAction });

    fireEvent.click(
      screen.getByRole("link", { name: "Profil ansehen: Ben Erwartet" }),
    );

    expect(onRosterAction).not.toHaveBeenCalled();
  });

  it("stays available in a block the caller may only view", () => {
    renderRoster({
      attendanceWebEnabled: false,
      studentProfileHref: profileHref,
    });

    expect(
      screen.getAllByRole("link", { name: /^Profil ansehen/ }),
    ).toHaveLength(2);
    expect(screen.queryByRole("button", { name: "Einchecken" })).toBeNull();
  });

  it("is absent without the right to open the profile", () => {
    renderRoster({});

    expect(screen.queryByRole("link", { name: /Profil ansehen/ })).toBeNull();
  });
});
