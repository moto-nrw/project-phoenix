import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TimetableRosterContent } from "./timetable-roster";
import type { TimetableRoster } from "~/lib/timetable-operations-types";

// #3809: a school may allow completing a block some minutes before its
// planned end. The locked button names that earlier time, not the end.

const roster: TimetableRoster = {
  instance: {
    id: "instance-1",
    title: "Fußball",
    status: "active",
    isSpontaneous: false,
    activeGroupId: "group-1",
    roomId: "room-1",
    roomName: "Turnhalle",
    date: "2099-09-09",
    startTime: "14:00",
    endTime: "16:00",
    canComplete: false,
    completeAvailableAt: "2099-09-09T15:45:00+02:00",
  },
  rows: [],
  pickupTimesLoaded: true,
};

describe("TimetableRosterContent complete lead (#3809)", () => {
  it("names the time from which the block can be completed", () => {
    render(
      <TimetableRosterContent
        addStudentResults={[]}
        addStudentSearch=""
        attendanceWebEnabled
        isAddingStudent={false}
        isCompletingInstance={false}
        isConfirmingExpected={false}
        roster={roster}
        showTimetableCounts={false}
        canAddUnplanned={false}
        onAddStudent={vi.fn()}
        onComplete={vi.fn()}
        onConfirmExpected={vi.fn()}
        onRosterAction={vi.fn()}
        onSearchChange={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Beenden ab 15:45" }),
    ).toBeDisabled();
  });
});
