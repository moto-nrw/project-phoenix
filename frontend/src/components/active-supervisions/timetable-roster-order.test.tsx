import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TimetableRosterContent } from "./timetable-roster";
import type {
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";

// #4353: the children in the room stand above every other list, planned and
// unplanned alike. #4356: class and group carry their own icon, like the
// room cards.

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

function walkIn(id: string, name: string, present: boolean) {
  return rosterRow(id, name, {
    planned: false,
    isUnplanned: true,
    currentlyPresent: present,
    status: "present",
    visitId: `v${id}`,
  });
}

function roster(
  rows: TimetableRosterRow[],
  isSpontaneous = false,
): TimetableRoster {
  return {
    instance: {
      id: "instance-1",
      title: "Malen",
      status: "active",
      isSpontaneous,
      activeGroupId: "group-1",
      roomId: "room-1",
      roomName: "Kunstraum",
      date: "2026-09-09",
      startTime: "11:00",
      endTime: "13:00",
      canComplete: false,
      completeAvailableAt: "",
    },
    rows,
    pickupTimesLoaded: true,
  };
}

function renderRoster(value: TimetableRoster) {
  render(
    <TimetableRosterContent
      addStudentResults={[]}
      addStudentSearch=""
      attendanceWebEnabled={false}
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

function sectionTitles(): string[] {
  return Array.from(
    document.querySelectorAll("section > div:first-child > span"),
  ).map((node) => node.textContent ?? "");
}

describe("TimetableRosterContent section order (#4353)", () => {
  it("lists planned and unplanned children in the room before every other list", () => {
    renderRoster(
      roster([
        rosterRow("1", "Anna Planned", { currentlyPresent: true }),
        rosterRow("2", "Ben Expected"),
        rosterRow("3", "Cara Absent", { status: "absent" }),
        rosterRow("4", "Dana Gone", { status: "present" }),
        walkIn("5", "Emil Walk-in", true),
      ]),
    );

    expect(sectionTitles()).toEqual([
      "Anwesend (1)",
      "Ungeplant (1)",
      "Erwartet (1)",
      "Entschuldigt / Abwesend (1)",
      "Nicht mehr im Raum (1)",
    ]);
  });

  it("puts the participants of a spontaneous block above the children who left", () => {
    renderRoster(
      roster(
        [walkIn("1", "Emma Meyer", true), walkIn("2", "Paul Becker", false)],
        true,
      ),
    );

    expect(sectionTitles()).toEqual([
      "Teilnehmende (1)",
      "Nicht mehr im Raum (1)",
    ]);
  });
});

describe("TimetableRosterContent student meta (#4356)", () => {
  it("shows class and group as separate lines with an icon each", () => {
    renderRoster(roster([rosterRow("1", "Anna", { currentlyPresent: true })]));

    for (const label of ["2a", "Gruppe: Sonnengruppe"]) {
      const line = screen.getByText(label).parentElement;
      expect(line?.querySelector("svg")).not.toBeNull();
    }
  });

  it("keeps the unplanned marker and leaves out missing values", () => {
    renderRoster(
      roster([
        rosterRow("1", "Emil", {
          planned: false,
          isUnplanned: true,
          currentlyPresent: true,
          status: "present",
          visitId: "v1",
          groupName: "",
        }),
      ]),
    );

    expect(screen.getByText("2a")).toBeInTheDocument();
    expect(screen.getByText("ungeplant")).toBeInTheDocument();
    expect(screen.queryByText(/^Gruppe:/)).not.toBeInTheDocument();
  });
});
