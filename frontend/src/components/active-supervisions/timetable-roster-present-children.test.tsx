import { fireEvent, render, screen } from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PresentChildrenPicker } from "./present-children-picker";
import { TimetableRosterContent } from "./timetable-roster";
import type { TimetableRoster } from "~/lib/timetable-operations-types";

const mocks = vi.hoisted(() => ({ fetchStudents: vi.fn() }));

vi.mock("~/lib/student-api", () => ({ fetchStudents: mocks.fetchStudents }));

const PICKER_TITLE = "Anwesende Kinder hinzufügen";

function roster(overrides: Partial<TimetableRoster> = {}): TimetableRoster {
  return {
    instance: {
      id: "instance-1",
      title: "Fußball-AG",
      status: "active",
      isSpontaneous: true,
      activeGroupId: "group-1",
      roomId: "room-1",
      roomName: "Turnhalle",
      date: "2026-09-09",
      startTime: "12:00",
      endTime: "13:00",
      canComplete: false,
      completeAvailableAt: "2026-09-09T11:00:00Z",
    },
    rows: [],
    pickupTimesLoaded: true,
    ...overrides,
  };
}

function renderRoster(
  props: Partial<React.ComponentProps<typeof TimetableRosterContent>> = {},
) {
  return render(
    <TimetableRosterContent
      addStudentResults={[]}
      addStudentSearch=""
      attendanceWebEnabled
      isAddingStudent={false}
      isCompletingInstance={false}
      isConfirmingExpected={false}
      roster={roster()}
      showTimetableCounts={false}
      onAddStudent={vi.fn()}
      onAddPresentStudents={vi.fn().mockResolvedValue(true)}
      presentChildrenPicker={PresentChildrenPicker}
      onComplete={vi.fn()}
      onConfirmExpected={vi.fn()}
      onRosterAction={vi.fn()}
      onSearchChange={vi.fn()}
      {...props}
    />,
  );
}

describe("TimetableRosterContent present children (#3824)", () => {
  beforeEach(() => {
    mocks.fetchStudents.mockReset();
    mocks.fetchStudents.mockResolvedValue({ students: [] });
  });

  it("opens the picker from the head action", () => {
    renderRoster();

    expect(screen.queryByText(PICKER_TITLE)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Anwesende Kinder" }));
    expect(screen.getByText(PICKER_TITLE)).toBeInTheDocument();
  });

  it("opens the picker once by itself after a spontaneous start", () => {
    const onPresentPickerAutoOpened = vi.fn();
    renderRoster({
      presentPickerAutoOpen: true,
      onPresentPickerAutoOpened,
    });

    expect(screen.getByText(PICKER_TITLE)).toBeInTheDocument();
    expect(onPresentPickerAutoOpened).toHaveBeenCalledTimes(1);
  });

  it("offers no picker without the right to check children in", () => {
    const onPresentPickerAutoOpened = vi.fn();
    renderRoster({
      roster: roster({ canEditAttendance: false, canOperate: false }),
      presentPickerAutoOpen: true,
      onPresentPickerAutoOpened,
    });

    expect(
      screen.queryByRole("button", { name: "Anwesende Kinder" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(PICKER_TITLE)).not.toBeInTheDocument();
    // The intent is spent either way, so it cannot fire on a later roster.
    expect(onPresentPickerAutoOpened).toHaveBeenCalledTimes(1);
  });

  it("offers no picker where unplanned children are not allowed", () => {
    renderRoster({ canAddUnplanned: false });

    expect(
      screen.queryByRole("button", { name: "Anwesende Kinder" }),
    ).not.toBeInTheDocument();
  });

  it("offers no picker without a bulk handler", () => {
    renderRoster({ onAddPresentStudents: undefined });

    expect(
      screen.queryByRole("button", { name: "Anwesende Kinder" }),
    ).not.toBeInTheDocument();
  });

  it("offers no picker when the page passes no picker dialog", () => {
    renderRoster({ presentChildrenPicker: undefined });

    expect(
      screen.queryByRole("button", { name: "Anwesende Kinder" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(PICKER_TITLE)).not.toBeInTheDocument();
  });
});
