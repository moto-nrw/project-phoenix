import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { TEST_CLOCK_TODAY } from "~/test/clock";

import {
  RestOfDayNotSavedError,
  TimetableRosterContent,
  runRestOfDayExcusalRequest,
} from "./timetable-roster";
import type {
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";

const { patchAttendance, saveStudentPartialAbsence } = vi.hoisted(() => ({
  patchAttendance: vi.fn(),
  saveStudentPartialAbsence: vi.fn(),
}));

vi.mock("~/lib/timetable-operations-api", () => ({
  timetableOperationsApi: { patchAttendance },
}));

vi.mock("~/lib/student-partial-absences-api", () => ({
  saveStudentPartialAbsence,
}));

const row: TimetableRosterRow = {
  studentId: "42",
  studentName: "Mia Beispiel",
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
};

const roster: TimetableRoster = {
  instance: {
    id: "instance-1",
    title: "Lernzeit",
    status: "active",
    isSpontaneous: false,
    activeGroupId: "group-1",
    roomId: "room-1",
    roomName: "Raum 1",
    date: TEST_CLOCK_TODAY,
    startTime: "11:30",
    endTime: "12:30",
    canComplete: false,
    completeAvailableAt: `${TEST_CLOCK_TODAY}T10:30:00Z`,
  },
  rows: [row],
  pickupTimesLoaded: true,
};

function renderRoster(
  onRosterAction: (action: string, row: TimetableRosterRow) => Promise<void>,
  onExcuseRestOfDay?: (row: TimetableRosterRow) => Promise<void>,
  value: TimetableRoster = roster,
) {
  render(
    <TimetableRosterContent
      addStudentResults={[]}
      addStudentSearch=""
      attendanceWebEnabled
      isAddingStudent={false}
      isCompletingInstance={false}
      isConfirmingExpected={false}
      roster={value}
      showTimetableCounts={false}
      canAddUnplanned={false}
      onAddStudent={vi.fn()}
      onComplete={vi.fn()}
      onConfirmExpected={vi.fn()}
      onRosterAction={onRosterAction}
      onExcuseRestOfDay={onExcuseRestOfDay}
      onSearchChange={vi.fn()}
    />,
  );
}

describe("Entschuldigt in Aktuelle Aufsicht (#3166)", () => {
  beforeEach(() => {
    patchAttendance.mockReset();
    saveStudentPartialAbsence.mockReset();
  });

  it("offers rest of day without assignment when absence reports are allowed", async () => {
    const onRosterAction = vi.fn().mockResolvedValue(undefined);
    const onExcuseRestOfDay = vi.fn().mockResolvedValue(undefined);
    renderRoster(onRosterAction, onExcuseRestOfDay, {
      ...roster,
      canOperate: false,
      canEditAttendance: false,
      canReportAbsence: true,
    });
    fireEvent.click(screen.getByRole("button", { name: "Entschuldigt" }));
    fireEvent.click(screen.getByRole("button", { name: /Rest des Tages/ }));
    await waitFor(() => expect(onExcuseRestOfDay).toHaveBeenCalledWith(row));
    expect(onRosterAction).not.toHaveBeenCalled();
    expect(
      screen.queryByRole("button", { name: /^Beenden/ }),
    ).not.toBeInTheDocument();
  });

  it("excuses only this block right away without the rest-of-day right", async () => {
    const onRosterAction = vi.fn().mockResolvedValue(undefined);
    renderRoster(onRosterAction);

    fireEvent.click(screen.getByRole("button", { name: "Entschuldigt" }));

    await waitFor(() =>
      expect(onRosterAction).toHaveBeenCalledWith("excused", row),
    );
    expect(screen.queryByText("Rest des Tages")).not.toBeInTheDocument();
  });

  it("asks for the scope first and excuses only this block on „Nur dieser Block“", async () => {
    const onRosterAction = vi.fn().mockResolvedValue(undefined);
    const onExcuseRestOfDay = vi.fn().mockResolvedValue(undefined);
    renderRoster(onRosterAction, onExcuseRestOfDay);

    fireEvent.click(screen.getByRole("button", { name: "Entschuldigt" }));

    expect(screen.getByText("Mia Beispiel entschuldigen")).toBeInTheDocument();
    expect(onRosterAction).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: /Nur dieser Block/ }));

    await waitFor(() =>
      expect(onRosterAction).toHaveBeenCalledWith("excused", row),
    );
    expect(onExcuseRestOfDay).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(
        screen.queryByText("Mia Beispiel entschuldigen"),
      ).not.toBeInTheDocument(),
    );
  });

  it("hands „Rest des Tages“ to the rest-of-day handler with the block start", async () => {
    const onRosterAction = vi.fn().mockResolvedValue(undefined);
    const onExcuseRestOfDay = vi.fn().mockResolvedValue(undefined);
    renderRoster(onRosterAction, onExcuseRestOfDay);

    fireEvent.click(screen.getByRole("button", { name: "Entschuldigt" }));
    const restOfDay = screen.getByRole("button", { name: /Rest des Tages/ });
    expect(restOfDay).toHaveTextContent("Ab 11:30 Uhr");

    fireEvent.click(restOfDay);

    await waitFor(() => expect(onExcuseRestOfDay).toHaveBeenCalledWith(row));
    expect(onRosterAction).not.toHaveBeenCalled();
  });
});

describe("runRestOfDayExcusalRequest", () => {
  beforeEach(() => {
    patchAttendance.mockReset();
    saveStudentPartialAbsence.mockReset();
  });

  it("excuses the block and records a partial absence from the block start", async () => {
    patchAttendance.mockResolvedValue(undefined);
    saveStudentPartialAbsence.mockResolvedValue({});

    await runRestOfDayExcusalRequest(roster.instance, "42");

    expect(patchAttendance).toHaveBeenCalledWith("instance-1", "42", {
      status: "absent",
      substatus: "excused",
    });
    expect(saveStudentPartialAbsence).toHaveBeenCalledWith(
      "42",
      null,
      TEST_CLOCK_TODAY,
      "11:30",
    );
    expect(patchAttendance.mock.invocationCallOrder[0]).toBeLessThan(
      saveStudentPartialAbsence.mock.invocationCallOrder[0] ?? 0,
    );
  });

  it("writes no partial absence when excusing the block fails", async () => {
    patchAttendance.mockRejectedValue(new Error("boom"));

    await expect(
      runRestOfDayExcusalRequest(roster.instance, "42"),
    ).rejects.toThrow("boom");
    expect(saveStudentPartialAbsence).not.toHaveBeenCalled();
  });

  it("reports that only the block was excused when the partial absence fails", async () => {
    patchAttendance.mockResolvedValue(undefined);
    saveStudentPartialAbsence.mockRejectedValue(new Error("conflict"));

    await expect(
      runRestOfDayExcusalRequest(roster.instance, "42"),
    ).rejects.toBeInstanceOf(RestOfDayNotSavedError);
  });
});
