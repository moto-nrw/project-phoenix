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
    const onAttendancePatch = vi.fn().mockResolvedValue(undefined);
    const { rerender } = render(
      <InstanceDetailSlideOver
        instance={instance({ status: "active", isLive: true })}
        onClose={vi.fn()}
        onLifecycleAction={onLifecycleAction}
        onAttendancePatch={onAttendancePatch}
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
    fireEvent.click(
      screen.getAllByRole("button", { name: "Als anwesend markieren" })[0]!,
    );
    await waitFor(() =>
      expect(onAttendancePatch).toHaveBeenCalledWith("42", "21", {
        status: "present",
        substatus: null,
      }),
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: "Als fehlend markieren" })[0]!,
    );
    await waitFor(() =>
      expect(onAttendancePatch).toHaveBeenCalledWith("42", "21", {
        status: "absent",
        substatus: null,
      }),
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: "Status zurücksetzen" })[0]!,
    );
    await waitFor(() =>
      expect(onAttendancePatch).toHaveBeenCalledWith("42", "22", {
        status: "expected",
        substatus: null,
        note: null,
      }),
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

  it("renders completed, empty, and detailed attendance states", () => {
    const onClose = vi.fn();
    const { rerender } = render(
      <InstanceDetailSlideOver
        instance={instance({
          status: "completed",
          staff: [],
          staffCount: 0,
          absentStaffCount: 0,
          students: [],
          studentIds: [],
          expectedStudentsCount: 0,
          presentStudentsCount: 0,
          notes: undefined,
          conflictWarnings: [],
          roomName: "",
        })}
        onClose={onClose}
        onLifecycleAction={vi.fn()}
      />,
    );

    expect(screen.getByText("Niemand zugeordnet")).toBeInTheDocument();
    expect(screen.getByText("Kein Personal zugeordnet.")).toBeInTheDocument();
    expect(screen.getByText("Keine Kinder geplant.")).toBeInTheDocument();
    expect(
      screen.getByText("Diese Aktivität ist bereits abgeschlossen."),
    ).toBeInTheDocument();
    expect(screen.getByText("Raum #3")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Schließen" }));
    expect(onClose).toHaveBeenCalledOnce();

    rerender(
      <InstanceDetailSlideOver
        instance={instance({
          status: "completed",
          students: [
            { studentId: "31", status: "absent", substatus: "late" },
            { studentId: "32", status: "absent", substatus: "excused" },
            { studentId: "33", status: "absent", substatus: "field_trip" },
            { studentId: "34", status: "absent", substatus: "other" },
          ],
          studentIds: ["31", "32", "33", "34"],
        })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
        onAttendancePatch={vi.fn()}
        studentNames={
          new Map([
            ["31", "Late Kind"],
            ["32", "Excused Kind"],
            ["33", "Trip Kind"],
            ["34", "Other Kind"],
          ])
        }
      />,
    );

    expect(screen.getByText(/Verspätet/)).toBeInTheDocument();
    expect(screen.getByText(/Entschuldigt/)).toBeInTheDocument();
    expect(screen.getByText(/Ausflug/)).toBeInTheDocument();
    expect(screen.getByText(/Sonstiges/)).toBeInTheDocument();
  });

  it("can back out of cancelled instance deletion confirmation", () => {
    const onDeleteCancelled = vi.fn();

    render(
      <InstanceDetailSlideOver
        instance={instance({ status: "cancelled" })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
        onDeleteCancelled={onDeleteCancelled}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Löschen/ }));
    expect(
      screen.getByRole("button", { name: /Löschen bestätigen/ }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Löschen abbrechen/ }));
    expect(
      screen.queryByRole("button", { name: /Löschen bestätigen/ }),
    ).not.toBeInTheDocument();
    expect(onDeleteCancelled).not.toHaveBeenCalled();
  });
});
