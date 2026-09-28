import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TimetableRosterContent } from "./timetable-roster";
import type { Occupancy } from "~/lib/activity-occupancy";
import type { TimetableRoster } from "~/lib/timetable-operations-types";

// #3634: the roster head of a running block shows its children against the
// activity's limit and, above it, „Überbucht" with the tablet hint.

const roster: TimetableRoster = {
  instance: {
    id: "instance-1",
    title: "Fußball",
    status: "active",
    isSpontaneous: false,
    activeGroupId: "group-1",
    roomId: "room-1",
    roomName: "Turnhalle",
    date: "2026-09-09",
    startTime: "13:00",
    endTime: "14:00",
    canComplete: false,
    completeAvailableAt: "2026-09-09T14:00:00Z",
  },
  rows: [],
  pickupTimesLoaded: true,
};

function renderHead(occupancy?: Occupancy) {
  render(
    <TimetableRosterContent
      addStudentResults={[]}
      addStudentSearch=""
      attendanceWebEnabled={false}
      isAddingStudent={false}
      isCompletingInstance={false}
      isConfirmingExpected={false}
      roster={roster}
      showTimetableCounts={false}
      occupancy={occupancy}
      canAddUnplanned={false}
      onAddStudent={vi.fn()}
      onComplete={vi.fn()}
      onConfirmExpected={vi.fn()}
      onRosterAction={vi.fn()}
      onSearchChange={vi.fn()}
    />,
  );
}

describe("TimetableRosterContent occupancy (#3634)", () => {
  it("names an overbooked block and explains the tablet stop", () => {
    renderHead({ count: 66, limit: 45 });

    expect(screen.getByText("66 / 45 Kinder")).toBeInTheDocument();
    expect(screen.getByText("Überbucht")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Mehr Kinder als erlaubt (höchstens 45). Am Tablet kann sich jetzt kein Kind anmelden. Das geht wieder unter 45 Kindern oder mit höherer Grenze.",
      ),
    ).toBeInTheDocument();
  });

  it("shows a full block without warning", () => {
    renderHead({ count: 45, limit: 45 });

    expect(screen.getByText("45 / 45 Kinder")).toBeInTheDocument();
    expect(screen.queryByText("Überbucht")).not.toBeInTheDocument();
    expect(
      screen.queryByText(/Mehr Kinder als erlaubt/),
    ).not.toBeInTheDocument();
  });

  it("shows a block below the limit without warning", () => {
    renderHead({ count: 12, limit: 45 });

    expect(screen.getByText("12 / 45 Kinder")).toBeInTheDocument();
    expect(screen.queryByText("Überbucht")).not.toBeInTheDocument();
  });

  it("keeps the head unchanged without a limit", () => {
    renderHead({ count: 66, limit: null });

    expect(screen.queryByText(/Kinder$/)).not.toBeInTheDocument();
    expect(screen.queryByText("Überbucht")).not.toBeInTheDocument();
  });
});
