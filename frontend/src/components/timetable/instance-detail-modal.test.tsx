import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import useSWR from "swr";

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { token: "test-token" } },
    status: "authenticated",
    update: vi.fn(),
  })),
}));

import { InstanceDetailModal } from "./instance-detail-modal";
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
    canComplete: true,
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

describe("InstanceDetailModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAttendanceWebEnabled).mockReturnValue(true);
    vi.mocked(useShowTimetableCounts).mockReturnValue(true);
  });

  it("renders nothing without a selected instance", () => {
    const { container } = render(
      <InstanceDetailModal
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
      <InstanceDetailModal
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

  it("only renders students returned by the participant endpoint in read-only mode", () => {
    vi.mocked(useSWR).mockReturnValue({
      data: {
        studentNames: new Map([["21", "Max Sichtbar"]]),
        staffNames: new Map(),
      },
    } as ReturnType<typeof useSWR>);

    render(
      <InstanceDetailModal
        instance={instance()}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
        canManage={false}
        fetchParticipantNames
      />,
    );

    expect(screen.getByText("Max Sichtbar")).toBeInTheDocument();
    expect(screen.queryByText("Kind #22")).not.toBeInTheDocument();
    expect(screen.queryByText("Kind #23")).not.toBeInTheDocument();
    expect(screen.queryByText("krank")).not.toBeInTheDocument();
  });

  // The header count already leaves these children out (#1747). Listing them
  // under "Erwartet" with an "abmelden" action would contradict that count and
  // write attendance for a day the child is not in care at all.
  it("groups children who are not in care today and drops their actions", () => {
    render(
      <InstanceDetailModal
        instance={instance({
          students: [
            { studentId: "21", status: "expected", careDayStatus: "scheduled" },
            {
              studentId: "24",
              status: "expected",
              careDayStatus: "not_scheduled",
            },
            { studentId: "25", status: "expected", careDayStatus: "cancelled" },
          ],
          studentIds: ["21", "24", "25"],
          expectedStudentsCount: 1,
          notScheduledStudentsCount: 2,
          presentStudentsCount: 0,
        })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
        onAttendancePatch={vi.fn()}
        studentNames={
          new Map([
            ["21", "Max Erwartet"],
            ["24", "Nina Ohne Betreuung"],
            ["25", "Ole Abgemeldet"],
          ])
        }
        editDeferred={false}
      />,
    );

    // Group header plus the row label of the not-booked child.
    expect(screen.getAllByText("Heute nicht eingeplant")).toHaveLength(2);
    expect(screen.getByText("Heute abgemeldet")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Max Erwartet abmelden" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Nina Ohne Betreuung/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Ole Abgemeldet/ }),
    ).not.toBeInTheDocument();
  });

  // A sick/excused report flips every still-expected slot of the day to
  // "absent", including the slots of children the care plan never booked that
  // weekday. The backend hands those rows a "not_scheduled" verdict until the
  // block ends and undoes them; showing them under "Fehlt" would claim an
  // absence from care that was never owed (#1747).
  it("groups a status-day absence on an unbooked day as not scheduled", () => {
    render(
      <InstanceDetailModal
        instance={instance({
          students: [
            {
              studentId: "24",
              status: "absent",
              substatus: "sick",
              careDayStatus: "not_scheduled",
            },
            { studentId: "25", status: "absent", careDayStatus: "unknown" },
          ],
          studentIds: ["24", "25"],
          expectedStudentsCount: 0,
          notScheduledStudentsCount: 1,
          presentStudentsCount: 0,
        })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
        onAttendancePatch={vi.fn()}
        studentNames={
          new Map([
            ["24", "Nina Ohne Betreuung"],
            ["25", "Ole Fehlt Wirklich"],
          ])
        }
        editDeferred={false}
      />,
    );

    // Group header plus the row label of the falsely absent child.
    expect(screen.getAllByText(/Heute nicht eingeplant/)).toHaveLength(2);
    expect(
      screen.queryByRole("button", { name: /Nina Ohne Betreuung/ }),
    ).not.toBeInTheDocument();
    // The genuine absence keeps its own group and its own label.
    expect(screen.getByText("Ole Fehlt Wirklich")).toBeInTheDocument();
    expect(screen.getAllByText("Abgemeldet").length).toBeGreaterThan(0);
  });

  it("marks spontaneous instances in the detail header", () => {
    render(
      <InstanceDetailModal
        instance={instance({ isSpontaneous: true })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
      />,
    );

    expect(screen.getByText("Spontan gestartet")).toBeInTheDocument();
  });

  it("locks complete until the planned end", () => {
    render(
      <InstanceDetailModal
        instance={instance({
          status: "active",
          isLive: true,
          canComplete: false,
          completeAvailableAt: "2099-05-04T13:00:00+02:00",
        })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
      />,
    );

    const button = screen.getByRole("button", { name: "Beenden ab 13:00" });
    expect(button).toBeDisabled();
  });

  it("completes an active instance", async () => {
    const onLifecycleAction = vi.fn().mockResolvedValue(undefined);
    render(
      <InstanceDetailModal
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
      <InstanceDetailModal
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

  // Not booked into care today is a plan, not a prophecy: the child can still
  // walk in, and this view is where the supervisor standing in the room records
  // it (#1747 review). Only "anwesend" survives here — "abmelden" and
  // "zuruecksetzen" would write attendance for a day the child was never
  // expected on.
  it("keeps the check-in action for a child who turns up unplanned", async () => {
    const onAttendancePatch = vi.fn().mockResolvedValue(undefined);

    render(
      <InstanceDetailModal
        instance={instance({
          status: "active",
          isLive: true,
          students: [
            {
              studentId: "24",
              status: "expected",
              careDayStatus: "not_scheduled",
            },
          ],
          studentIds: ["24"],
          expectedStudentsCount: 0,
          notScheduledStudentsCount: 1,
          presentStudentsCount: 0,
        })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
        onAttendancePatch={onAttendancePatch}
        studentNames={new Map([["24", "Nina Ohne Betreuung"]])}
        editDeferred={false}
      />,
    );

    expect(
      screen.queryByRole("button", {
        name: "Nina Ohne Betreuung als fehlend markieren",
      }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: "Status von Nina Ohne Betreuung zurücksetzen",
      }),
    ).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Nina Ohne Betreuung als anwesend markieren",
      }),
    );
    await waitFor(() =>
      expect(onAttendancePatch).toHaveBeenCalledWith("42", "24", {
        status: "present",
        substatus: null,
      }),
    );
  });

  it("patches active attendance states", async () => {
    const onAttendancePatch = vi.fn().mockResolvedValue(undefined);

    render(
      <InstanceDetailModal
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
      <InstanceDetailModal
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
      <InstanceDetailModal
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
      <InstanceDetailModal
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
      <InstanceDetailModal
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
      <InstanceDetailModal
        instance={instance({
          activityGroupId: "7",
          isSpontaneous: false,
          date: "2099-05-04",
        })}
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
      <InstanceDetailModal
        instance={instance({
          activityGroupId: "7",
          isSpontaneous: false,
          date: "2099-05-04",
        })}
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
      <InstanceDetailModal
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

  it("uses single delete for past occurrences of a series", async () => {
    const onDeleteCancelled = vi.fn().mockResolvedValue(undefined);
    const onDeleteFollowing = vi.fn().mockResolvedValue(undefined);

    render(
      <InstanceDetailModal
        instance={instance({
          activityGroupId: "7",
          isSpontaneous: false,
          date: "2020-05-04",
          status: "cancelled",
        })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
        onDeleteCancelled={onDeleteCancelled}
        onDeleteFollowing={onDeleteFollowing}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Löschen/ }));
    // "Ab jetzt dauerhaft" beendet die Serie ab dem Termindatum — das lehnt
    // das Backend für vergangene Daten ab, also darf die Auswahl gar nicht
    // erst erscheinen.
    expect(
      screen.queryByRole("dialog", { name: "Wiederholenden Termin löschen" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Abgesagten Termin löschen?")).toBeInTheDocument();

    fireEvent.click(confirmDialogButton("Löschen"));
    await waitFor(() =>
      expect(onDeleteCancelled).toHaveBeenCalledWith(
        expect.objectContaining({ id: "42", activityGroupId: "7" }),
      ),
    );
    expect(onDeleteFollowing).not.toHaveBeenCalled();
  });

  it("switches an open recurring-delete dialog to single delete after Berlin midnight", async () => {
    vi.useFakeTimers({ toFake: ["Date", "setInterval", "clearInterval"] });
    vi.setSystemTime(new Date("2026-05-04T21:59:30Z"));
    try {
      render(
        <InstanceDetailModal
          instance={instance({ activityGroupId: "7", date: "2026-05-04" })}
          onClose={vi.fn()}
          onLifecycleAction={vi.fn()}
          onDeleteCancelled={vi.fn()}
          onDeleteFollowing={vi.fn()}
        />,
      );

      fireEvent.click(screen.getByRole("button", { name: /Löschen/ }));
      expect(
        screen.getByRole("dialog", { name: "Wiederholenden Termin löschen" }),
      ).toBeInTheDocument();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(60_000);
      });

      expect(
        screen.queryByRole("dialog", { name: "Wiederholenden Termin löschen" }),
      ).not.toBeInTheDocument();
      expect(screen.getByText("Termin löschen?")).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("does not open recurring delete during the hook refresh interval after Berlin midnight", () => {
    vi.useFakeTimers({ toFake: ["Date", "setInterval", "clearInterval"] });
    vi.setSystemTime(new Date("2026-05-04T21:59:30Z"));
    try {
      render(
        <InstanceDetailModal
          instance={instance({ activityGroupId: "7", date: "2026-05-04" })}
          onClose={vi.fn()}
          onLifecycleAction={vi.fn()}
          onDeleteCancelled={vi.fn()}
          onDeleteFollowing={vi.fn()}
        />,
      );

      vi.setSystemTime(new Date("2026-05-04T22:00:01Z"));
      fireEvent.click(screen.getByRole("button", { name: /Löschen/ }));

      expect(
        screen.queryByRole("dialog", { name: "Wiederholenden Termin löschen" }),
      ).not.toBeInTheDocument();
      expect(screen.getByText("Termin löschen?")).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("rechecks Berlin's date before ending a series from an already open scope dialog", () => {
    vi.useFakeTimers({ toFake: ["Date", "setInterval", "clearInterval"] });
    vi.setSystemTime(new Date("2026-05-04T21:59:30Z"));
    const onDeleteFollowing = vi.fn();
    try {
      render(
        <InstanceDetailModal
          instance={instance({ activityGroupId: "7", date: "2026-05-04" })}
          onClose={vi.fn()}
          onLifecycleAction={vi.fn()}
          onDeleteCancelled={vi.fn()}
          onDeleteFollowing={onDeleteFollowing}
        />,
      );

      fireEvent.click(screen.getByRole("button", { name: /Löschen/ }));
      vi.setSystemTime(new Date("2026-05-04T22:00:01Z"));
      fireEvent.click(screen.getByText("Ab jetzt dauerhaft"));

      expect(onDeleteFollowing).not.toHaveBeenCalled();
      expect(
        screen.queryByRole("dialog", { name: "Wiederholenden Termin löschen" }),
      ).not.toBeInTheDocument();
      expect(screen.getByText("Termin löschen?")).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("does not open a second confirmation while a scope deletion spans Berlin midnight", async () => {
    vi.useFakeTimers({ toFake: ["Date", "setInterval", "clearInterval"] });
    vi.setSystemTime(new Date("2026-05-04T21:59:30Z"));
    let resolveDeletion: (() => void) | undefined;
    const onDeleteFollowing = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          resolveDeletion = resolve;
        }),
    );
    try {
      render(
        <InstanceDetailModal
          instance={instance({ activityGroupId: "7", date: "2026-05-04" })}
          onClose={vi.fn()}
          onLifecycleAction={vi.fn()}
          onDeleteCancelled={vi.fn()}
          onDeleteFollowing={onDeleteFollowing}
        />,
      );

      fireEvent.click(screen.getByRole("button", { name: /Löschen/ }));
      fireEvent.click(screen.getByText("Ab jetzt dauerhaft"));
      await waitFor(() => expect(onDeleteFollowing).toHaveBeenCalledOnce());

      await act(async () => {
        await vi.advanceTimersByTimeAsync(60_000);
      });

      expect(
        screen.getByRole("dialog", { name: "Wiederholenden Termin löschen" }),
      ).toBeInTheDocument();
      expect(screen.queryByText("Termin löschen?")).not.toBeInTheDocument();

      await act(async () => {
        resolveDeletion?.();
      });

      expect(
        screen.queryByRole("dialog", { name: "Wiederholenden Termin löschen" }),
      ).not.toBeInTheDocument();
      expect(screen.queryByText("Termin löschen?")).not.toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("renders completed, empty, and detailed attendance states", async () => {
    const onClose = vi.fn();
    const { rerender } = render(
      <InstanceDetailModal
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
    expect(
      screen.queryByRole("button", { name: "Wieder öffnen" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Raum #3")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Schließen" }));
    // Das Kit-Modal ruft onClose erst nach der Exit-Animation (250ms) auf.
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());

    rerender(
      <InstanceDetailModal
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
    expect(
      screen.queryByRole("button", { name: /als anwesend markieren|abmelden/ }),
    ).not.toBeInTheDocument();
  });

  it("explains why an offering-sourced occurrence has no children", () => {
    const { rerender } = render(
      <InstanceDetailModal
        instance={instance({
          students: [],
          studentIds: [],
          expectedStudentsCount: 0,
          presentStudentsCount: 0,
          emptyRosterReason: {
            kind: "before_offering_start",
            phaseName: "Schuljahr 2026/27",
            serviceStartDate: "2026-08-13",
          },
        })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
      />,
    );

    expect(
      screen
        .getByText(
          "Dieser Termin liegt vor dem Betreuungsbeginn am 13.08.2026. Die Kinder aus den ausgewählten Angeboten werden erst ab diesem Tag übernommen.",
        )
        .closest('[role="status"]'),
    ).not.toBeNull();
    expect(screen.queryByText("Keine Kinder geplant.")).not.toBeInTheDocument();

    rerender(
      <InstanceDetailModal
        instance={instance({
          students: [],
          studentIds: [],
          expectedStudentsCount: 0,
          presentStudentsCount: 0,
          emptyRosterReason: { kind: "offering_source_empty" },
        })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
      />,
    );
    expect(
      screen.getByText(
        "Aus den ausgewählten Angeboten wurden für diesen Termin keine Kinder übernommen. Das kann an den gebuchten Wochentagen, den gewählten Filtern oder geänderten Anmeldungen liegen.",
      ),
    ).toBeInTheDocument();
  });

  it("hides itself while another overlay is stacked on top", () => {
    const onClose = vi.fn();
    render(
      <InstanceDetailModal
        instance={instance({})}
        onClose={onClose}
        onLifecycleAction={vi.fn()}
        suspended
      />,
    );

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
  });

  it("shows the Regeltermin OriginChip only when the instance stems from one", () => {
    const { rerender } = render(
      <InstanceDetailModal
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
      <InstanceDetailModal
        instance={instance({ activityGroupId: undefined })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
      />,
    );

    expect(screen.queryByText(/aus Regeltermin/)).not.toBeInTheDocument();
  });

  it("labels the substitution jump action 'Vertretung bearbeiten' and links to /vertretung", () => {
    render(
      <InstanceDetailModal
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
      <InstanceDetailModal
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

describe("Personalpool-Affordanz (#1884)", () => {
  it("zeigt 'Person hinzuziehen' für einen zukünftigen geplanten Block", () => {
    const onOpenPool = vi.fn();
    render(
      <InstanceDetailModal
        instance={instance({ date: "2099-05-04" })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
        onOpenPool={onOpenPool}
        canManageStaffPool
      />,
    );

    const button = screen.getByRole("button", { name: /Person hinzuziehen/ });
    fireEvent.click(button);
    expect(onOpenPool).toHaveBeenCalledWith(
      expect.objectContaining({ id: "42" }),
    );
  });

  it("zeigt einen neutralen Nur-Lese-Einstieg ohne Verwaltungsrecht", () => {
    const onOpenPool = vi.fn();
    render(
      <InstanceDetailModal
        instance={instance({ date: "2099-05-04" })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
        onOpenPool={onOpenPool}
      />,
    );

    const button = screen.getByRole("button", {
      name: /Personalpool ansehen/,
    });
    fireEvent.click(button);
    expect(onOpenPool).toHaveBeenCalledWith(
      expect.objectContaining({ id: "42" }),
    );
    expect(
      screen.queryByRole("button", { name: /Person hinzuziehen/ }),
    ).not.toBeInTheDocument();
  });

  it("versteckt die Affordanz für vergangene Blöcke", () => {
    render(
      <InstanceDetailModal
        instance={instance({ date: "2020-05-04" })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
        onOpenPool={vi.fn()}
      />,
    );
    expect(
      screen.queryByRole("button", { name: /Person hinzuziehen/ }),
    ).not.toBeInTheDocument();
  });

  it("versteckt die Affordanz für abgeschlossene Blöcke", () => {
    render(
      <InstanceDetailModal
        instance={instance({ date: "2099-05-04", status: "completed" })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
        onOpenPool={vi.fn()}
      />,
    );
    expect(
      screen.queryByRole("button", { name: /Person hinzuziehen/ }),
    ).not.toBeInTheDocument();
  });

  it("zeigt Wieder öffnen nur bei canReopen", () => {
    render(
      <InstanceDetailModal
        instance={instance({
          date: "2099-05-04",
          status: "completed",
          canReopen: true,
        })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Wieder öffnen" }),
    ).toBeInTheDocument();
  });

  it("versteckt die Affordanz ohne onOpenPool-Handler", () => {
    render(
      <InstanceDetailModal
        instance={instance({ date: "2099-05-04" })}
        onClose={vi.fn()}
        onLifecycleAction={vi.fn()}
      />,
    );
    expect(
      screen.queryByRole("button", { name: /Person hinzuziehen/ }),
    ).not.toBeInTheDocument();
  });
});
