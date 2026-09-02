import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

afterEach(() => {
  vi.useRealTimers();
});

import { TimetableRosterContent } from "./timetable-roster";
import type {
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";

// A class-wide arrival day exception (#2962) reaches the roster as an
// informational warning. It renders as a plain line at the child, never as an
// amber planning note, and disappears once the child is present.

function rosterRow(
  studentId: string,
  studentName: string,
  overrides: Partial<TimetableRosterRow> = {},
): TimetableRosterRow {
  return {
    studentId,
    studentName,
    schoolClass: "4a",
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

function roster(rows: TimetableRosterRow[]): TimetableRoster {
  return {
    instance: {
      id: "instance-1",
      title: "GT 4",
      status: "active",
      isSpontaneous: false,
      activeGroupId: "group-1",
      roomId: "room-1",
      roomName: "Raum 1",
      date: "2026-09-02",
      startTime: "12:45",
      endTime: "13:45",
      canComplete: false,
      completeAvailableAt: "2026-09-02T13:45:00Z",
    },
    rows,
    pickupTimesLoaded: true,
  };
}

const classExceptionWarning: TimetableRosterRow["warnings"] = [
  {
    kind: "class_arrival_exception",
    message: "Kommt heute um 12:45 Uhr (Klasse 4a: Unterricht fällt aus)",
    expectedArrival: "12:45",
    slotStart: "12:45",
    expectedGroupId: null,
    expectedGroupName: null,
    currentEducationGroupId: null,
  },
];

function renderRoster(value: TimetableRoster) {
  return render(
    <TimetableRosterContent
      addStudentResults={[]}
      addStudentSearch=""
      attendanceWebEnabled
      isAddingStudent={false}
      isCompletingInstance={false}
      isConfirmingExpected={false}
      roster={value}
      showTimetableCounts
      canAddUnplanned={false}
      onAddStudent={vi.fn()}
      onComplete={vi.fn()}
      onConfirmExpected={vi.fn()}
      onRosterAction={vi.fn()}
      onSearchChange={vi.fn()}
    />,
  );
}

describe("TimetableRosterContent class arrival exception", () => {
  it("shows the class line under Erwartet instead of an amber warning", () => {
    renderRoster(
      roster([
        rosterRow("1", "Ausfall Kind", { warnings: classExceptionWarning }),
      ]),
    );

    const line = screen.getByText(
      "Kommt heute um 12:45 Uhr (Klasse 4a: Unterricht fällt aus)",
    );
    expect(line).toBeInTheDocument();
    expect(line.className).not.toContain("text-moto-amber-strong");
    expect(screen.queryByText("Kommt später")).not.toBeInTheDocument();
    expect(screen.getByText("Erwartet (1)")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Einchecken" }),
    ).toBeInTheDocument();
  });

  it("drops the line once the child is present", () => {
    renderRoster(
      roster([
        rosterRow("1", "Schon da", {
          currentlyPresent: true,
          status: "present",
          visitId: "visit-1",
          warnings: classExceptionWarning,
        }),
      ]),
    );

    expect(
      screen.queryByText(
        "Kommt heute um 12:45 Uhr (Klasse 4a: Unterricht fällt aus)",
      ),
    ).not.toBeInTheDocument();
  });

  it("tells the reader that single check-in stays possible under Kommt später", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-09-02T12:50:00+02:00"));
    renderRoster(
      roster([
        rosterRow("1", "Später Kind", {
          warnings: [
            {
              kind: "arrival_after_slot_start",
              message:
                "Erwartete Ankunft liegt nach dem Start dieser Betreuung.",
              expectedArrival: "23:59",
              slotStart: "12:45",
              expectedGroupId: null,
              expectedGroupName: null,
              currentEducationGroupId: null,
            },
          ],
        }),
      ]),
    );

    expect(
      screen.getByText(
        "Diese Kinder kommen laut Plan später. Bei „Erwartete bestätigen“ sind sie nicht dabei. Kommt ein Kind früher, checken Sie es hier einzeln ein.",
      ),
    ).toBeInTheDocument();
  });
});
