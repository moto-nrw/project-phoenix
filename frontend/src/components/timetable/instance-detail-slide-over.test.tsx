import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { InstanceDetailSlideOver } from "./instance-detail-slide-over";
import type { EnrichedInstance } from "~/lib/timetable-types";
import {
  useAttendanceWebEnabled,
  useShowTimetableCounts,
} from "~/lib/tenant-context";

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
    notScheduledStudentsCount: 0,
    presentStudentsCount: 1,
    requiredStaffCount: 1,
    assignedStaffCount: 1,
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

function confirmDialogButton(name: string) {
  const dialogs = screen.getAllByRole("dialog");
  return within(dialogs[dialogs.length - 1]!).getByRole("button", { name });
}

async function expectNoUnhandledRejection(
  action: () => Promise<void>,
): Promise<void> {
  const unhandledRejections: unknown[] = [];
  const recordUnhandledRejection = (reason: unknown) => {
    unhandledRejections.push(reason);
  };
  process.on("unhandledRejection", recordUnhandledRejection);
  try {
    await action();
    await new Promise<void>((resolve) => setTimeout(resolve, 0));
    expect(unhandledRejections).toEqual([]);
  } finally {
    process.off("unhandledRejection", recordUnhandledRejection);
  }
}

describe("InstanceDetailSlideOver", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAttendanceWebEnabled).mockReturnValue(true);
    vi.mocked(useShowTimetableCounts).mockReturnValue(true);
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

    expect(
      screen.queryByRole("button", { name: /als anwesend markieren/ }),
    ).not.toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Max Erwartet abmelden" }),
    );
    await waitFor(() =>
      expect(onAttendancePatch).toHaveBeenCalledWith("42", "21", {
        status: "absent",
        substatus: "excused",
      }),
    );
  });

  it("marks spontaneous instances in the detail header", () => {
    render(
      <InstanceDetailSlideOver
        instance={instance({ isSpontaneous: true })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
      />,
    );

    expect(screen.getByText("Spontan gestartet")).toBeInTheDocument();
  });

  it("completes an active instance", async () => {
    const onLifecycleAction = vi.fn().mockResolvedValue(undefined);
    render(
      <InstanceDetailSlideOver
        instance={instance({ status: "active", isLive: true })}
        onClose={vi.fn()}
        onLifecycleAction={onLifecycleAction}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Beenden/ }));
    expect(screen.getByText("Termin beenden?")).toBeInTheDocument();
    fireEvent.click(confirmDialogButton("Beenden"));
    await waitFor(() =>
      expect(onLifecycleAction).toHaveBeenCalledWith("complete"),
    );
    await waitFor(() =>
      expect(screen.queryByText("Termin beenden?")).not.toBeInTheDocument(),
    );
  });

  it("cancels an active instance", async () => {
    const onLifecycleAction = vi.fn().mockResolvedValue(undefined);
    render(
      <InstanceDetailSlideOver
        instance={instance({ status: "active", isLive: true })}
        onClose={vi.fn()}
        onLifecycleAction={onLifecycleAction}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Absagen/ }));
    expect(screen.getByText("Termin absagen?")).toBeInTheDocument();
    fireEvent.click(confirmDialogButton("Absagen"));
    await waitFor(() =>
      expect(onLifecycleAction).toHaveBeenCalledWith("cancel"),
    );
    await waitFor(() =>
      expect(screen.queryByText("Termin absagen?")).not.toBeInTheDocument(),
    );
  });

  it("patches active attendance states", async () => {
    const onAttendancePatch = vi.fn().mockResolvedValue(undefined);

    render(
      <InstanceDetailSlideOver
        instance={instance({ status: "active", isLive: true })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
        onAttendancePatch={onAttendancePatch}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", {
        name: "Kind #21 als anwesend markieren",
      }),
    );
    await waitFor(() =>
      expect(onAttendancePatch).toHaveBeenCalledWith("42", "21", {
        status: "present",
        substatus: null,
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Kind #21 als fehlend markieren" }),
    );
    await waitFor(() =>
      expect(onAttendancePatch).toHaveBeenCalledWith("42", "21", {
        status: "absent",
        substatus: null,
      }),
    );
    fireEvent.click(
      screen.getByRole("button", {
        name: "Status von Kind #22 zurücksetzen",
      }),
    );
    await waitFor(() =>
      expect(onAttendancePatch).toHaveBeenCalledWith("42", "22", {
        status: "expected",
        substatus: null,
        note: null,
      }),
    );
  });

  it("hides attendance mutations when web attendance is disabled", () => {
    vi.mocked(useAttendanceWebEnabled).mockReturnValue(false);
    const onAttendancePatch = vi.fn();

    render(
      <InstanceDetailSlideOver
        instance={instance({ status: "active", isLive: true })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
        onAttendancePatch={onAttendancePatch}
      />,
    );

    expect(
      screen.queryByRole("button", { name: "Als anwesend markieren" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Als fehlend markieren" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Status zurücksetzen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Beenden" }),
    ).not.toBeInTheDocument();
    expect(onAttendancePatch).not.toHaveBeenCalled();
  });

  it("contains a reported lifecycle rejection and resets pending state", async () => {
    const onLifecycleAction = vi
      .fn()
      .mockRejectedValue(new Error("already reported lifecycle failure"));
    render(
      <InstanceDetailSlideOver
        instance={instance({ status: "active", isLive: true })}
        onClose={vi.fn()}
        onLifecycleAction={onLifecycleAction}
      />,
    );

    await expectNoUnhandledRejection(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Beenden" }));
      fireEvent.click(confirmDialogButton("Beenden"));

      await waitFor(() =>
        expect(onLifecycleAction).toHaveBeenCalledWith("complete"),
      );
      await waitFor(() =>
        expect(screen.getByRole("button", { name: "Beenden" })).toBeEnabled(),
      );
    });
  });

  it("contains a reported attendance rejection and resets the row action", async () => {
    const onAttendancePatch = vi
      .fn()
      .mockRejectedValue(new Error("already reported attendance failure"));
    render(
      <InstanceDetailSlideOver
        instance={instance({ status: "active", isLive: true })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
        onAttendancePatch={onAttendancePatch}
      />,
    );

    await expectNoUnhandledRejection(async () => {
      const attendanceButton = screen.getByRole("button", {
        name: "Kind #21 als anwesend markieren",
      });
      fireEvent.click(attendanceButton);

      await waitFor(() => expect(onAttendancePatch).toHaveBeenCalledOnce());
      await waitFor(() => expect(attendanceButton).toBeEnabled());
    });
  });

  it("handles cancelled fallback student states", async () => {
    const onLifecycleAction = vi.fn().mockResolvedValue(undefined);
    const onDeleteCancelled = vi.fn().mockResolvedValue(undefined);

    render(
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
    expect(screen.getByText("Abgesagten Termin löschen?")).toBeInTheDocument();
    fireEvent.click(confirmDialogButton("Löschen"));
    await waitFor(() =>
      expect(onDeleteCancelled).toHaveBeenCalledWith(
        expect.objectContaining({ id: "42" }),
      ),
    );
  });

  it("asks for delete scope on recurring planned instances", async () => {
    const onDeleteCancelled = vi.fn().mockResolvedValue(undefined);
    const onDeleteFollowing = vi.fn().mockResolvedValue(undefined);

    render(
      <InstanceDetailSlideOver
        instance={instance({ activityGroupId: "7", isSpontaneous: false })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
        onDeleteCancelled={onDeleteCancelled}
        onDeleteFollowing={onDeleteFollowing}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Löschen/ }));
    expect(
      screen.getByRole("dialog", { name: "Wiederholenden Termin löschen" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByText("Ab jetzt dauerhaft"));
    await waitFor(() =>
      expect(onDeleteFollowing).toHaveBeenCalledWith(
        expect.objectContaining({ id: "42", activityGroupId: "7" }),
      ),
    );
    expect(onDeleteCancelled).not.toHaveBeenCalled();
  });

  it("contains a reported recurring-delete rejection and keeps the scope dialog usable", async () => {
    const onDeleteFollowing = vi
      .fn()
      .mockRejectedValue(new Error("already reported delete failure"));

    render(
      <InstanceDetailSlideOver
        instance={instance({ activityGroupId: "7", isSpontaneous: false })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
        onDeleteCancelled={vi.fn()}
        onDeleteFollowing={onDeleteFollowing}
      />,
    );

    await expectNoUnhandledRejection(async () => {
      fireEvent.click(screen.getByRole("button", { name: /Löschen/ }));
      const followingOption = screen
        .getByText("Ab jetzt dauerhaft")
        .closest("button");
      expect(followingOption).not.toBeNull();
      fireEvent.click(followingOption!);

      await waitFor(() => expect(onDeleteFollowing).toHaveBeenCalledOnce());
      await waitFor(() => expect(followingOption).toBeEnabled());
      expect(
        screen.getByRole("dialog", { name: "Wiederholenden Termin löschen" }),
      ).toBeInTheDocument();
    });
  });

  it("uses single delete for spontaneous instances even when they have an activity group", async () => {
    const onDeleteCancelled = vi.fn().mockResolvedValue(undefined);
    const onDeleteFollowing = vi.fn().mockResolvedValue(undefined);

    render(
      <InstanceDetailSlideOver
        instance={instance({ activityGroupId: "7", isSpontaneous: true })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
        onDeleteCancelled={onDeleteCancelled}
        onDeleteFollowing={onDeleteFollowing}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Löschen/ }));
    expect(
      screen.queryByRole("dialog", { name: "Wiederholenden Termin löschen" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Termin löschen?")).toBeInTheDocument();

    fireEvent.click(confirmDialogButton("Löschen"));
    await waitFor(() =>
      expect(onDeleteCancelled).toHaveBeenCalledWith(
        expect.objectContaining({
          id: "42",
          activityGroupId: "7",
          isSpontaneous: true,
        }),
      ),
    );
    expect(onDeleteFollowing).not.toHaveBeenCalled();
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
          notScheduledStudentsCount: 0,
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

  it("shows the Regeltermin OriginChip only when the instance stems from one", () => {
    const { rerender } = render(
      <InstanceDetailSlideOver
        instance={instance({
          activityGroupId: "9",
          date: "2026-05-04", // Monday
          startTime: "12:00",
          title: "Mensa",
        })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
      />,
    );

    expect(
      screen.getByText("aus Regeltermin Mensa, montags 12:00"),
    ).toBeInTheDocument();

    rerender(
      <InstanceDetailSlideOver
        instance={instance({ activityGroupId: undefined })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
      />,
    );

    expect(screen.queryByText(/aus Regeltermin/)).not.toBeInTheDocument();
  });

  it("labels the substitution jump action 'Vertretung bearbeiten' and links to /vertretung", () => {
    render(
      <InstanceDetailSlideOver
        instance={instance({
          status: "planned",
          staff: [
            {
              staffId: "11",
              isPrimary: true,
              isAbsent: true,
              isSubstitute: false,
            },
          ],
        })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
      />,
    );

    const link = screen.getByRole("link", { name: /Vertretung bearbeiten/ });
    expect(link).toHaveAttribute(
      "href",
      expect.stringContaining("/vertretung?d=2026-05-04&block=42"),
    );
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
    expect(screen.getByText("Abgesagten Termin löschen?")).toBeInTheDocument();

    fireEvent.click(confirmDialogButton("Abbrechen"));
    expect(
      screen.queryByText("Abgesagten Termin löschen?"),
    ).not.toBeInTheDocument();
    expect(onDeleteCancelled).not.toHaveBeenCalled();
  });
});
