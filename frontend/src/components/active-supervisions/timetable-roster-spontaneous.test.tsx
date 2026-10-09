import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TimetableRosterContent } from "./timetable-roster";
import type {
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";

// #3921: a spontaneous block has no children of its own. Every child came
// in unplanned, so the planned-only figures (Anwesend, Erwartet, Abwesend)
// are always 0 there. The head shows the same name as the list instead, and
// no invented end time while the block runs.

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

function walkIn(
  id: string,
  name: string,
  present: boolean,
): TimetableRosterRow {
  return rosterRow(id, name, {
    planned: false,
    isUnplanned: true,
    currentlyPresent: present,
    status: "present",
    visitId: `v${id}`,
  });
}

function spontaneous(status: "active" | "completed"): TimetableRoster {
  return {
    instance: {
      id: "instance-9",
      title: "Tanzen",
      status,
      isSpontaneous: true,
      activeGroupId: "group-9",
      roomId: "room-9",
      roomName: "Bewegungsraum",
      date: "2026-09-09",
      startTime: "15:16",
      endTime: status === "active" ? "16:16" : "17:40",
      canComplete: true,
      completeAvailableAt: "2026-09-09T13:16:00Z",
    },
    rows: [
      walkIn("1", "Emma Meyer", true),
      walkIn("2", "Emil Fischer", true),
      walkIn("3", "Paul Becker", false),
    ],
    pickupTimesLoaded: true,
  };
}

function renderSpontaneous(status: "active" | "completed") {
  render(
    <TimetableRosterContent
      addStudentResults={[]}
      addStudentSearch=""
      attendanceWebEnabled={false}
      isAddingStudent={false}
      isCompletingInstance={false}
      isConfirmingExpected={false}
      roster={spontaneous(status)}
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

function headFigure(label: string): string | null | undefined {
  return screen.getByText(label, { selector: "span" }).previousSibling
    ?.textContent;
}

describe("TimetableRosterContent for a spontaneous block (#3921)", () => {
  it("counts the children in the room under the name of the list", () => {
    renderSpontaneous("active");

    expect(headFigure("Teilnehmende")).toBe("2");
    expect(headFigure("Gegangen")).toBe("1");
    expect(screen.getByText("Teilnehmende (2)")).toBeInTheDocument();
    for (const label of ["Anwesend", "Erwartet", "Abwesend", "Ungeplant"]) {
      expect(
        screen.queryByText(label, { selector: "span" }),
      ).not.toBeInTheDocument();
    }
  });

  it("shows no invented end while the block runs", () => {
    renderSpontaneous("active");

    expect(screen.getByText(/seit 15:16/)).toBeInTheDocument();
    expect(screen.queryByText(/16:16/)).not.toBeInTheDocument();
  });

  it("shows the recorded end once the block is over", () => {
    renderSpontaneous("completed");

    expect(screen.getByText(/15:16-17:40/)).toBeInTheDocument();
  });
});
