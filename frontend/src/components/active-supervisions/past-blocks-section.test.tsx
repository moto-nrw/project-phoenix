import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PastBlocksSection } from "./past-blocks-section";
import type {
  PlannedTimetableInstance,
  TimetableRoster,
} from "~/lib/timetable-operations-types";

const { plannedNowMock, rosterMock } = vi.hoisted(() => ({
  plannedNowMock: vi.fn(),
  rosterMock: vi.fn(),
}));

vi.mock("~/lib/timetable-operations-api", () => ({
  timetableOperationsApi: {
    plannedNow: plannedNowMock,
    roster: rosterMock,
  },
}));

function pastBlock(
  overrides: Partial<PlannedTimetableInstance>,
): PlannedTimetableInstance {
  return {
    id: "instance-1",
    date: "2026-08-16",
    title: "Aufsicht 2B",
    roomId: "room-1",
    roomName: "Raum 2B",
    startTime: "12:45",
    endTime: "14:30",
    status: "completed",
    expectedStudentsCount: 8,
    notScheduledStudentsCount: 0,
    presentStudentsCount: 6,
    minutesUntilStart: -180,
    assignedStaffIds: ["staff-1", "staff-2"],
    isAssigned: true,
    isPrimary: false,
    isSubstitute: false,
    isAbsent: false,
    rosterPreview: [],
    isOverdue: false,
    ...overrides,
  };
}

const completedRoster: TimetableRoster = {
  instance: {
    id: "instance-1",
    title: "Aufsicht 2B",
    status: "completed",
    isSpontaneous: false,
    activeGroupId: null,
    roomId: "room-1",
    roomName: "Raum 2B",
    date: "2026-08-16",
    startTime: "12:45",
    endTime: "14:30",
    canComplete: false,
    completeAvailableAt: "",
  },
  pickupTimesLoaded: true,
  rows: [
    {
      studentId: "student-1",
      studentName: "Mia Bauer",
      schoolClass: "2a",
      groupName: "Sternengruppe",
      planned: true,
      isUnplanned: false,
      currentlyPresent: false,
      visitId: "visit-1",
      status: "present",
      substatus: null,
      note: null,
      checkedInAt: "2026-08-16T12:50:00Z",
      checkedOutAt: "2026-08-16T14:25:00Z",
      visitEntryTime: null,
      pickupTime: "14:30",
      warnings: [],
      careDayStatus: "scheduled",
      parallelPresentIn: null,
    },
    {
      studentId: "student-2",
      studentName: "Tom Weber",
      schoolClass: "2b",
      groupName: "Sternengruppe",
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
    },
  ],
};

describe("PastBlocksSection", () => {
  beforeEach(() => {
    plannedNowMock.mockReset();
    rosterMock.mockReset();
  });

  it("stays collapsed and does not fetch until opened", () => {
    render(<PastBlocksSection />);

    expect(
      screen.getByText("Beendete und abgelaufene Blöcke"),
    ).toBeInTheDocument();
    expect(plannedNowMock).not.toHaveBeenCalled();
  });

  it("loads past blocks on expand and labels completed and never-started ones", async () => {
    plannedNowMock.mockResolvedValue([
      pastBlock({}),
      pastBlock({
        id: "instance-2",
        title: "Lernzeit",
        status: "planned",
        startTime: "10:00",
        endTime: "11:00",
      }),
    ]);

    render(<PastBlocksSection />);
    fireEvent.click(
      screen.getByRole("button", { name: /Beendete und abgelaufene Blöcke/ }),
    );

    await waitFor(() =>
      expect(screen.getByText("Aufsicht 2B")).toBeInTheDocument(),
    );
    expect(plannedNowMock).toHaveBeenCalledWith({ scope: "past" });
    expect(screen.getByText("Beendet")).toBeInTheDocument();
    expect(screen.getByText("Nicht gestartet")).toBeInTheDocument();
    // Sorted chronologically: the 10:00 block renders before the 12:45 one.
    const titles = screen
      .getAllByRole("heading", { level: 3 })
      .map((el) => el.textContent);
    expect(titles).toEqual(["Lernzeit", "Aufsicht 2B"]);
  });

  it("shows an empty state when nothing has finished yet", async () => {
    plannedNowMock.mockResolvedValue([]);

    render(<PastBlocksSection />);
    fireEvent.click(
      screen.getByRole("button", { name: /Beendete und abgelaufene Blöcke/ }),
    );

    await waitFor(() =>
      expect(
        screen.getByText(
          "Heute sind noch keine Blöcke beendet oder abgelaufen.",
        ),
      ).toBeInTheDocument(),
    );
  });

  it("lazy-loads the roster with final statuses for a completed block", async () => {
    plannedNowMock.mockResolvedValue([pastBlock({})]);
    rosterMock.mockResolvedValue(completedRoster);

    render(<PastBlocksSection />);
    fireEvent.click(
      screen.getByRole("button", { name: /Beendete und abgelaufene Blöcke/ }),
    );
    await waitFor(() =>
      expect(screen.getByText("Aufsicht 2B")).toBeInTheDocument(),
    );
    expect(rosterMock).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Kinder" }));

    await waitFor(() =>
      expect(screen.getByText("Mia Bauer")).toBeInTheDocument(),
    );
    expect(rosterMock).toHaveBeenCalledWith("instance-1");
    expect(screen.getByText("Gegangen")).toBeInTheDocument();
    expect(screen.getByText("Nicht erschienen")).toBeInTheDocument();
    expect(screen.getByText("Gehzeit: 14:30")).toBeInTheDocument();
    expect(screen.getByText("Gehzeit: —")).toBeInTheDocument();
  });

  it("distinguishes a failed pickup lookup from a missing pickup time", async () => {
    plannedNowMock.mockResolvedValue([pastBlock({})]);
    rosterMock.mockResolvedValue({
      ...completedRoster,
      pickupTimesLoaded: false,
    });

    render(<PastBlocksSection />);
    fireEvent.click(
      screen.getByRole("button", { name: /Beendete und abgelaufene Blöcke/ }),
    );
    await waitFor(() =>
      expect(screen.getByText("Aufsicht 2B")).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "Kinder" }));

    await waitFor(() =>
      expect(
        screen.getByText(
          "Die Gehzeiten konnten nicht geladen werden. Die Anwesenheitsliste bleibt verfügbar.",
        ),
      ).toBeInTheDocument(),
    );
    expect(screen.getAllByText("Gehzeit: Nicht geladen")).toHaveLength(2);
    expect(screen.queryByText("Gehzeit: —")).not.toBeInTheDocument();
  });

  it("shows the never-started hint instead of per-child statuses", async () => {
    plannedNowMock.mockResolvedValue([
      pastBlock({ status: "planned", title: "Lernzeit" }),
    ]);
    rosterMock.mockResolvedValue({
      ...completedRoster,
      instance: { ...completedRoster.instance, status: "planned" },
    });

    render(<PastBlocksSection />);
    fireEvent.click(
      screen.getByRole("button", { name: /Beendete und abgelaufene Blöcke/ }),
    );
    await waitFor(() =>
      expect(screen.getByText("Lernzeit")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "Kinder" }));

    await waitFor(() =>
      expect(
        screen.getByText(
          "Keine Anwesenheit erfasst: Der Block wurde nicht gestartet.",
        ),
      ).toBeInTheDocument(),
    );
    expect(screen.getByText("Mia Bauer")).toBeInTheDocument();
    expect(screen.queryByText("Gegangen")).not.toBeInTheDocument();
  });

  it("surfaces a load error", async () => {
    plannedNowMock.mockRejectedValue(new Error("kaputt"));

    render(<PastBlocksSection />);
    fireEvent.click(
      screen.getByRole("button", { name: /Beendete und abgelaufene Blöcke/ }),
    );

    await waitFor(() =>
      expect(
        screen.getByText(
          "Vergangene Blöcke konnten nicht geladen werden. Bitte später erneut versuchen.",
        ),
      ).toBeInTheDocument(),
    );
  });
});
