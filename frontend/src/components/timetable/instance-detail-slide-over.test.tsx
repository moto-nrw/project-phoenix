import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { InstanceDetailSlideOver } from "./instance-detail-slide-over";
import type { EnrichedInstance } from "~/lib/timetable-types";

function instance(overrides: Partial<EnrichedInstance> = {}): EnrichedInstance {
  return {
    id: "42",
    date: "2026-05-04",
    startTime: "12:00",
    endTime: "13:00",
    title: "Mensa",
    notes: "ohne Nuesse",
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityType: "activity",
    roomId: "3",
    roomName: "Mensa",
    staff: [
      {
        staffId: "11",
        isPrimary: true,
        isAbsent: false,
        isSubstitute: false,
      },
      {
        staffId: "12",
        isPrimary: false,
        isAbsent: true,
        isSubstitute: false,
      },
    ],
    students: [
      { studentId: "21", status: "expected" },
      { studentId: "22", status: "present" },
      { studentId: "23", status: "absent", substatus: "sick", note: "krank" },
    ],
    studentIds: ["21", "22", "23"],
    staffCount: 2,
    absentStaffCount: 1,
    expectedStudentsCount: 3,
    presentStudentsCount: 1,
    conflictWarnings: [
      {
        kind: "staff",
        resourceId: "11",
        message: "Doppelt belegt",
        canOverride: true,
      },
    ],
    ...overrides,
  };
}

describe("InstanceDetailSlideOver", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing without a selected instance", () => {
    const { container } = render(
      <InstanceDetailSlideOver
        instance={null}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("renders details and executes lifecycle/edit/repeat/attendance actions", async () => {
    const onLifecycleAction = vi.fn().mockResolvedValue(undefined);
    const onEdit = vi.fn();
    const onRepeat = vi.fn();
    const onAttendancePatch = vi.fn().mockResolvedValue(undefined);

    render(
      <InstanceDetailSlideOver
        instance={instance()}
        onClose={vi.fn()}
        onLifecycleAction={onLifecycleAction}
        onEdit={onEdit}
        onRepeat={onRepeat}
        onAttendancePatch={onAttendancePatch}
        staffNames={
          new Map([
            ["11", "Ada Staff"],
            ["12", "Ben Absent"],
          ])
        }
        studentNames={
          new Map([
            ["21", "Max Erwartet"],
            ["22", "Mia Anwesend"],
            ["23", "Tom Krank"],
          ])
        }
        editDeferred={false}
      />,
    );

    expect(screen.getByRole("heading", { name: /Mensa/ })).toBeVisible();
    expect(document.body).toHaveTextContent("Doppelt belegt");
    expect(screen.getByText("Ada Staff")).toBeInTheDocument();
    expect(screen.getByText("Ben Absent")).toBeInTheDocument();
    expect(screen.getByText("Max Erwartet")).toBeInTheDocument();
    expect(screen.getByText("Mia Anwesend")).toBeInTheDocument();
    expect(screen.getByText("Tom Krank")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Bearbeiten/ }));
    expect(onEdit).toHaveBeenCalledWith(expect.objectContaining({ id: "42" }));
    fireEvent.click(screen.getByRole("button", { name: /Wiederholen/ }));
    expect(onRepeat).toHaveBeenCalledWith(
      expect.objectContaining({ id: "42" }),
    );

    expect(onAttendancePatch).not.toHaveBeenCalled();
  });

  it("handles active, cancelled and fallback student states", async () => {
    const onLifecycleAction = vi.fn().mockResolvedValue(undefined);
    const onDeleteCancelled = vi.fn().mockResolvedValue(undefined);
    const { rerender } = render(
      <InstanceDetailSlideOver
        instance={instance({ status: "active", isLive: true })}
        onClose={vi.fn()}
        onLifecycleAction={onLifecycleAction}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Beenden/ }));
    await waitFor(() =>
      expect(onLifecycleAction).toHaveBeenCalledWith("complete"),
    );
    fireEvent.click(screen.getByRole("button", { name: /Absagen/ }));
    await waitFor(() =>
      expect(onLifecycleAction).toHaveBeenCalledWith("cancel"),
    );

    rerender(
      <InstanceDetailSlideOver
        instance={instance({
          status: "cancelled",
          students: [],
          studentIds: ["99"],
          activityType: "external",
        })}
        onClose={vi.fn()}
        onLifecycleAction={onLifecycleAction}
        onDeleteCancelled={onDeleteCancelled}
        studentNames={new Map([["99", "Fallback Kind"]])}
      />,
    );

    expect(screen.getByText("Fallback Kind")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Löschen/ }));
    expect(
      screen.getByRole("button", { name: /Löschen bestätigen/ }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Löschen bestätigen/ }));
    await waitFor(() =>
      expect(onDeleteCancelled).toHaveBeenCalledWith(
        expect.objectContaining({ id: "42" }),
      ),
    );
  });
});
