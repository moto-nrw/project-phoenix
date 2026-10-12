import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TimetableRosterContent } from "./timetable-roster";
import type {
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";

// #3889: the page's search used to stop at the children in the room. A
// block's list holds expected, absent and departed children too; the search
// must find them there and show them under their status.

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

const ROWS = [
  rosterRow("1", "Emma Meyer", {
    currentlyPresent: true,
    status: "present",
  }),
  rosterRow("2", "Emil Fischer"),
  rosterRow("3", "Anna Emmerich", { status: "absent" }),
  rosterRow("4", "Paul Becker", { status: "present", visitId: "v4" }),
];

function roster(rows: TimetableRosterRow[]): TimetableRoster {
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
      startTime: "11:00",
      endTime: "13:30",
      canComplete: false,
      completeAvailableAt: "2026-09-09T11:30:00Z",
    },
    rows,
    pickupTimesLoaded: true,
  };
}

function renderRoster(
  rowFilter: ((row: TimetableRosterRow) => boolean) | null,
) {
  render(
    <TimetableRosterContent
      addStudentResults={[]}
      addStudentSearch=""
      attendanceWebEnabled={false}
      isAddingStudent={false}
      isCompletingInstance={false}
      isConfirmingExpected={false}
      roster={roster(ROWS)}
      showTimetableCounts
      canAddUnplanned={false}
      onAddStudent={vi.fn()}
      onComplete={vi.fn()}
      onConfirmExpected={vi.fn()}
      onRosterAction={vi.fn()}
      onSearchChange={vi.fn()}
      rowFilter={rowFilter}
    />,
  );
}

function section(title: RegExp): HTMLElement {
  const match = Array.from(document.querySelectorAll("section")).find((el) =>
    title.test(el.textContent ?? ""),
  );
  if (!match) throw new Error(`no section ${title}`);
  return match;
}

describe("TimetableRosterContent search (#3889)", () => {
  it("shows every child under its status without a search", () => {
    renderRoster(null);

    for (const name of [
      "Emma Meyer",
      "Emil Fischer",
      "Anna Emmerich",
      "Paul Becker",
    ]) {
      expect(screen.getByText(name)).toBeInTheDocument();
    }
  });

  it("finds expected, absent and departed children, each under its status", () => {
    renderRoster((row) => row.studentName.toLowerCase().includes("em"));

    expect(
      within(section(/^Anwesend/)).getByText("Emma Meyer"),
    ).toBeInTheDocument();
    expect(
      within(section(/^Erwartet/)).getByText("Emil Fischer"),
    ).toBeInTheDocument();
    expect(
      within(section(/^Entschuldigt \/ Abwesend/)).getByText("Anna Emmerich"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Paul Becker")).not.toBeInTheDocument();
    expect(screen.queryByText(/^Nicht mehr im Raum/)).not.toBeInTheDocument();
  });

  it("finds a child who already left the room", () => {
    renderRoster((row) => row.studentName.includes("Paul"));

    expect(
      within(section(/^Nicht mehr im Raum/)).getByText("Paul Becker"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Emma Meyer")).not.toBeInTheDocument();
  });

  it("keeps the head figures of the whole block while searching", () => {
    renderRoster((row) => row.studentName.includes("Paul"));

    // The section title reads „Anwesend (0)“; the head figure alone „Anwesend“.
    expect(screen.getByText("Anwesend").previousSibling).toHaveTextContent("1");
    expect(screen.getByText("Gegangen").previousSibling).toHaveTextContent("1");
  });

  it("says so when the search finds nobody in the block", () => {
    renderRoster(() => false);

    expect(screen.getByText("Keine Kinder gefunden")).toBeInTheDocument();
    expect(screen.queryByText("Emma Meyer")).not.toBeInTheDocument();
  });
});
