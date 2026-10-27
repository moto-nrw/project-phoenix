import { fireEvent, render, screen, within } from "@testing-library/react";
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

type RowFilter = ((row: TimetableRosterRow) => boolean) | null;

function rosterView(value: TimetableRoster, rowFilter: RowFilter = null) {
  return (
    <TimetableRosterContent
      rowFilter={rowFilter}
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
    />
  );
}

function renderRoster(value: TimetableRoster) {
  return render(rosterView(value));
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

describe("TimetableRosterContent collapsible sections (#4353)", () => {
  const rows = [
    rosterRow("1", "Anna Planned", { currentlyPresent: true }),
    rosterRow("2", "Ben Expected"),
    rosterRow("4", "Dana Gone", { status: "present" }),
  ];

  it("opens with the children who left folded and every other list open", () => {
    renderRoster(roster(rows));

    expect(
      screen.getByRole("button", { name: "Nicht mehr im Raum ausklappen" }),
    ).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("Dana Gone")).not.toBeInTheDocument();
    for (const title of ["Anwesend", "Erwartet"]) {
      expect(
        screen.getByRole("button", { name: `${title} einklappen` }),
      ).toHaveAttribute("aria-expanded", "true");
    }
    expect(screen.getByText("Anna Planned")).toBeInTheDocument();
    expect(screen.getByText("Ben Expected")).toBeInTheDocument();
  });

  it("folds and unfolds a list from the arrow in its head", () => {
    renderRoster(roster(rows));

    fireEvent.click(
      screen.getByRole("button", { name: "Nicht mehr im Raum ausklappen" }),
    );
    expect(screen.getByText("Dana Gone")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Anwesend einklappen" }),
    );
    expect(screen.queryByText("Anna Planned")).not.toBeInTheDocument();
    expect(screen.getByText("Anwesend (1)")).toBeInTheDocument();
  });

  it("folds and unfolds a list from a click on its title (#5021)", () => {
    renderRoster(roster(rows));

    fireEvent.click(screen.getByText("Nicht mehr im Raum (1)"));
    expect(screen.getByText("Dana Gone")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Nicht mehr im Raum einklappen" }),
    ).toHaveAttribute("aria-expanded", "true");

    fireEvent.click(screen.getByText("Nicht mehr im Raum (1)"));
    expect(screen.queryByText("Dana Gone")).not.toBeInTheDocument();
  });

  it("opens every list for a search and folds the children who left again after it", () => {
    const { rerender } = renderRoster(roster(rows));
    rerender(
      rosterView(roster(rows), (row) => row.studentName.startsWith("Dana")),
    );
    expect(screen.getByText("Dana Gone")).toBeInTheDocument();

    rerender(rosterView(roster(rows)));
    expect(screen.queryByText("Dana Gone")).not.toBeInTheDocument();
    expect(screen.getByText("Anna Planned")).toBeInTheDocument();
  });
});

describe("TimetableRosterContent student meta (#4356)", () => {
  it("shows class and group with an icon each on the line of the name", () => {
    renderRoster(roster([rosterRow("1", "Anna", { currentlyPresent: true })]));

    const nameLine = screen.getByText("Anna").parentElement as HTMLElement;
    for (const value of ["2a", "Sonnengruppe"]) {
      const entry = within(nameLine).getByText(value);
      expect(entry.querySelector("svg")).not.toBeNull();
    }
    expect(nameLine).toHaveTextContent(/Klasse 2a\s*Gruppe Sonnengruppe/);
    expect(screen.queryByText(/Gruppe:/)).not.toBeInTheDocument();
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

    const nameLine = screen.getByText("Emil").parentElement as HTMLElement;
    expect(within(nameLine).getByText("2a")).toBeInTheDocument();
    expect(within(nameLine).getByText("ungeplant")).toBeInTheDocument();
    expect(within(nameLine).queryByText("Gruppe")).not.toBeInTheDocument();
  });
});
