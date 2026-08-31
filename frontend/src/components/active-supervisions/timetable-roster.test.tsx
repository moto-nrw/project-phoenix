import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

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

function renderRoster(
  value: TimetableRoster,
  attendanceWebEnabled = false,
  onConfirmExpected: (rows: TimetableRosterRow[]) => Promise<void> = vi.fn(),
  showTimetableCounts = false,
) {
  render(
    <TimetableRosterContent
      addStudentResults={[]}
      addStudentSearch=""
      attendanceWebEnabled={attendanceWebEnabled}
      isAddingStudent={false}
      isCompletingInstance={false}
      isConfirmingExpected={false}
      roster={value}
      showTimetableCounts={showTimetableCounts}
      canAddUnplanned={false}
      onAddStudent={vi.fn()}
      onComplete={vi.fn()}
      onConfirmExpected={onConfirmExpected}
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

function arrivalWarning(
  expectedArrival: string,
): TimetableRosterRow["warnings"] {
  return [
    {
      kind: "arrival_after_slot_start",
      message: "Erwartete Ankunft liegt nach dem Start dieser Betreuung.",
      expectedArrival,
      slotStart: "12:45",
      expectedGroupId: null,
      expectedGroupName: null,
      currentEducationGroupId: null,
    },
  ];
}

describe("TimetableRosterContent late arrivals", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  const lateRoster = () =>
    roster(
      [
        rosterRow("1", "Pünktlich Kind", null),
        rosterRow("2", "Später Kind", null, {
          warnings: arrivalWarning("13:45"),
        }),
      ],
      true,
    );

  it("groups a child before the expected arrival under Kommt später", () => {
    vi.setSystemTime(new Date(2026, 7, 31, 13, 0));
    renderRoster(lateRoster(), true);

    const laterSection = screen
      .getByText("Kommt später")
      .closest("section") as HTMLElement;
    expect(within(laterSection).getByText("Später Kind")).toBeInTheDocument();
    expect(
      within(laterSection).getByText("Kommt um 13:45 Uhr"),
    ).toBeInTheDocument();

    const expectedSection = screen
      .getByText("Erwartet")
      .closest("section") as HTMLElement;
    expect(
      within(expectedSection).queryByText("Später Kind"),
    ).not.toBeInTheDocument();

    // Individual check-in stays available for a child who arrives early.
    expect(
      within(laterSection).getByRole("button", { name: "Einchecken" }),
    ).toBeInTheDocument();
  });

  it("excludes late arrivals from the bulk expected confirmation", () => {
    vi.setSystemTime(new Date(2026, 7, 31, 13, 0));
    const onConfirmExpected = vi.fn().mockResolvedValue(undefined);
    renderRoster(lateRoster(), true, onConfirmExpected);

    fireEvent.click(
      screen.getByRole("button", { name: /Erwartete bestätigen/ }),
    );

    expect(onConfirmExpected).toHaveBeenCalledTimes(1);
    const rows = onConfirmExpected.mock.calls[0]?.[0] as TimetableRosterRow[];
    expect(rows.map((row) => row.studentId)).toEqual(["1"]);
  });

  it("counts late arrivals in their own header stat", () => {
    vi.setSystemTime(new Date(2026, 7, 31, 13, 0));
    renderRoster(lateRoster(), true, vi.fn(), true);

    const statLabel = screen.getByText("Kommt später");
    expect(
      within(statLabel.parentElement as HTMLElement).getByText("1"),
    ).toBeInTheDocument();
    // The section header carries its own count, so the exact-match stat label
    // above cannot be the section title ("Kommt später (1)").
    expect(screen.getByText("Kommt später (1)")).toBeInTheDocument();
  });

  it("moves the child to Erwartet when the minute clock crosses the arrival", () => {
    vi.setSystemTime(new Date(2026, 7, 31, 13, 44));
    renderRoster(lateRoster(), true);

    expect(screen.getByText("Kommt später")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(60_000);
    });

    expect(screen.queryByText("Kommt später")).not.toBeInTheDocument();
    const expectedSection = screen
      .getByText("Erwartet")
      .closest("section") as HTMLElement;
    expect(
      within(expectedSection).getByText("Später Kind"),
    ).toBeInTheDocument();
  });

  it("moves the child to Erwartet once the expected arrival is reached", () => {
    vi.setSystemTime(new Date(2026, 7, 31, 13, 45));
    renderRoster(lateRoster(), true);

    expect(screen.queryByText("Kommt später")).not.toBeInTheDocument();
    expect(screen.queryByText("Kommt um 13:45 Uhr")).not.toBeInTheDocument();
    const expectedSection = screen
      .getByText("Erwartet")
      .closest("section") as HTMLElement;
    expect(
      within(expectedSection).getByText("Später Kind"),
    ).toBeInTheDocument();
  });

  it("keeps other planning warnings visible in the started view", () => {
    vi.setSystemTime(new Date(2026, 7, 31, 13, 0));
    renderRoster(
      roster(
        [
          rosterRow("3", "Falsche Gruppe Kind", null, {
            warnings: [
              {
                kind: "template_class_mismatch",
                message:
                  "Kind passt nicht zur Klassengruppe der Betreuungsplan-Vorlage.",
                expectedArrival: null,
                slotStart: null,
                expectedGroupId: "12",
                expectedGroupName: "Klasse 2a",
                currentEducationGroupId: "13",
              },
            ],
          }),
        ],
        true,
      ),
      true,
    );

    expect(
      screen.getByText(
        "Kind passt nicht zur Klassengruppe der Betreuungsplan-Vorlage.",
      ),
    ).toBeInTheDocument();
  });
});
