import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PlannedNowSection } from "./planned-now-section";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

const plannedInstance: PlannedTimetableInstance = {
  id: "instance-1",
  date: "2026-05-10",
  title: "Hausaufgaben",
  roomId: "room-1",
  roomName: "Lernraum 2",
  startTime: "14:00",
  endTime: "15:00",
  status: "planned",
  expectedStudentsCount: 8,
  notScheduledStudentsCount: 0,
  presentStudentsCount: 0,
  minutesUntilStart: -5,
  assignedStaffIds: ["staff-1"],
  isAssigned: true,
  isPrimary: true,
  isSubstitute: false,
  isAbsent: false,
  rosterPreview: [
    {
      studentId: "student-1",
      studentName: "Mia Bauer",
      schoolClass: "2a",
      groupName: "Sternengruppe",
      planned: true,
      isUnplanned: false,
      currentlyPresent: false,
      visitId: null,
      status: "expected",
      substatus: null,
      note: null,
      checkedInAt: null,
      visitEntryTime: null,
      warnings: [],
      careDayStatus: "unknown",
    },
  ],
  isOverdue: true,
};

describe("PlannedNowSection", () => {
  it("renders an empty state when no planned instances are due", () => {
    render(
      <PlannedNowSection
        plannedNow={[]}
        isStartingInstance={null}
        onStart={vi.fn()}
      />,
    );

    expect(screen.getByText("Als Nächstes")).toBeInTheDocument();
    expect(
      screen.getByText("Keine geplante Betreuung in Sicht"),
    ).toBeInTheDocument();
  });

  it("renders planned instances and starts the selected one", () => {
    const onStart = vi.fn();

    const { rerender } = render(
      <PlannedNowSection
        plannedNow={[plannedInstance]}
        isStartingInstance="instance-1"
        onStart={onStart}
      />,
    );

    expect(
      screen.getByRole("button", { name: /Als Nächstes/ }),
    ).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("1 geplant")).toBeInTheDocument();
    expect(screen.getByText("8 Kinder")).toBeInTheDocument();
    expect(screen.getAllByText("Hausaufgaben").length).toBeGreaterThan(0);
    expect(screen.getByText("Lernraum 2")).toBeInTheDocument();
    expect(screen.getByText("Zuständig")).toBeInTheDocument();
    expect(screen.getByText("Überfällig")).toBeInTheDocument();
    expect(screen.getAllByText("Erwartet").length).toBeGreaterThan(0);
    expect(screen.getByText("Mia Bauer")).toBeInTheDocument();

    const startButton = screen.getByRole("button", { name: /Startet/i });
    expect(startButton).toBeDisabled();

    rerender(
      <PlannedNowSection
        plannedNow={[plannedInstance]}
        isStartingInstance={null}
        onStart={onStart}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /^Starten$/i }));
    expect(onStart).toHaveBeenCalledWith(plannedInstance);
  });

  it("keeps an early activity visible but locks start until the backend boundary", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-05-10T13:00:00+02:00"));
    try {
      render(
        <PlannedNowSection
          plannedNow={[
            {
              ...plannedInstance,
              canStart: false,
              startAvailableAt: "2026-05-10T13:45:00+02:00",
              startExpiresAt: "2026-05-10T15:00:00+02:00",
            },
          ]}
          isStartingInstance={null}
          onStart={vi.fn()}
        />,
      );
      const button = screen.getByRole("button", { name: "Starten ab 13:45" });
      expect(button).toBeDisabled();
    } finally {
      vi.useRealTimers();
    }
  });

  it("hides a planned activity after its start window has closed", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-05-10T15:00:00+02:00"));
    try {
      render(
        <PlannedNowSection
          plannedNow={[
            {
              ...plannedInstance,
              canStart: false,
              startAvailableAt: "2026-05-10T13:45:00+02:00",
              startExpiresAt: "2026-05-10T15:00:00+02:00",
            },
          ]}
          isStartingInstance={null}
          onStart={vi.fn()}
        />,
      );
      expect(screen.queryByText("Hausaufgaben")).not.toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("renders roster planning warnings", () => {
    render(
      <PlannedNowSection
        plannedNow={[
          {
            ...plannedInstance,
            rosterPreview: [
              {
                ...plannedInstance.rosterPreview[0]!,
                warnings: [
                  {
                    kind: "arrival_after_slot_start",
                    message:
                      "Erwartete Ankunft liegt nach dem Start dieser Betreuung.",
                    expectedArrival: "14:30",
                    slotStart: "14:00",
                    expectedGroupId: "12",
                    expectedGroupName: "Klasse 2a",
                    currentEducationGroupId: "13",
                  },
                ],
              },
            ],
          },
        ]}
        isStartingInstance={null}
        onStart={vi.fn()}
      />,
    );

    expect(screen.getByLabelText("1 Planungs-Hinweis")).toHaveAttribute(
      "title",
      "Erwartete Ankunft liegt nach dem Start dieser Betreuung.",
    );
    expect(
      screen.getByText(
        "Erwartete Ankunft liegt nach dem Start dieser Betreuung.",
      ),
    ).toBeInTheDocument();
  });

  it("does not crash when a roster preview row has no warnings array", () => {
    const legacyInstance = {
      ...plannedInstance,
      rosterPreview: [
        {
          ...plannedInstance.rosterPreview[0]!,
          warnings: undefined,
        },
      ],
    } as unknown as PlannedTimetableInstance;

    render(
      <PlannedNowSection
        plannedNow={[legacyInstance]}
        isStartingInstance={null}
        onStart={vi.fn()}
      />,
    );

    expect(screen.getByText("Mia Bauer")).toBeInTheDocument();
    expect(screen.queryByLabelText(/Planungs-Hinweis/)).toBeNull();
  });

  it("keeps future-only slots collapsed until opened", () => {
    render(
      <PlannedNowSection
        plannedNow={[
          {
            ...plannedInstance,
            isOverdue: false,
            minutesUntilStart: 90,
          },
        ]}
        isStartingInstance={null}
        onStart={vi.fn()}
      />,
    );

    const toggle = screen.getByRole("button", { name: /Als Nächstes/ });

    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("button", { name: /^Starten$/i })).toBeNull();

    fireEvent.click(toggle);

    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(
      screen.getByRole("button", { name: /^Starten$/i }),
    ).toBeInTheDocument();
  });

  it("keeps soon slots collapsed while another timetable session is active", () => {
    render(
      <PlannedNowSection
        plannedNow={[
          {
            ...plannedInstance,
            isOverdue: false,
            minutesUntilStart: 5,
          },
        ]}
        hasActiveTimetableSession
        isStartingInstance={null}
        onStart={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: /Als Nächstes/ }),
    ).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("button", { name: /^Starten$/i })).toBeNull();
  });

  it("renders multiple cards and on-time status labels", () => {
    render(
      <PlannedNowSection
        plannedNow={[
          {
            ...plannedInstance,
            id: "instance-1",
            isOverdue: false,
            minutesUntilStart: 10,
          },
          {
            ...plannedInstance,
            id: "instance-2",
            title: "AG Sport",
            isPrimary: false,
            isSubstitute: true,
          },
        ]}
        isStartingInstance={null}
        onStart={vi.fn()}
      />,
    );

    expect(screen.getByText("2 geplant")).toBeInTheDocument();
    expect(screen.getAllByText("AG Sport").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Startet gleich")).toHaveLength(1);
    expect(screen.getByText("Vertretung")).toBeInTheDocument();
  });

  it("sets children apart who are not in care today without hiding them", () => {
    render(
      <PlannedNowSection
        plannedNow={[
          {
            ...plannedInstance,
            // The counts are the backend's, not a roster derivation: it counts
            // only rows that are still expected AND scheduled today.
            expectedStudentsCount: 1,
            notScheduledStudentsCount: 1,
            rosterPreview: [
              plannedInstance.rosterPreview[0]!,
              {
                ...plannedInstance.rosterPreview[0]!,
                studentId: "student-2",
                studentName: "Jonas Weber",
                careDayStatus: "not_scheduled",
              },
            ],
          },
        ]}
        isStartingInstance={null}
        onStart={vi.fn()}
      />,
    );

    // Still listed, so a child who turns up anyway stays one tap from a
    // check-in — only the status label sets them apart.
    expect(screen.getByText("Jonas Weber")).toBeInTheDocument();
    expect(screen.getByText("Nicht eingeplant")).toBeInTheDocument();
    expect(
      screen.getByText("1 erwartet · 1 heute nicht eingeplant"),
    ).toBeInTheDocument();
  });

  it("counts an abgemeldetes Kind as not expected, like the backend does", () => {
    render(
      <PlannedNowSection
        plannedNow={[
          {
            ...plannedInstance,
            // The counts are the backend's, not a roster derivation: it counts
            // only rows that are still expected AND scheduled today.
            expectedStudentsCount: 1,
            notScheduledStudentsCount: 1,
            rosterPreview: [
              plannedInstance.rosterPreview[0]!,
              {
                ...plannedInstance.rosterPreview[0]!,
                studentId: "student-2",
                studentName: "Jonas Weber",
                careDayStatus: "cancelled",
              },
            ],
          },
        ]}
        isStartingInstance={null}
        onStart={vi.fn()}
      />,
    );

    // "cancelled" is left out of expected_students_count too — showing the
    // child as "Erwartet" here would contradict the header count and the full
    // roster, which groups both verdicts together (#1747).
    expect(screen.getByText("Jonas Weber")).toBeInTheDocument();
    expect(screen.getByText("Abgemeldet")).toBeInTheDocument();
    expect(
      screen.getByText("1 erwartet · 1 heute nicht eingeplant"),
    ).toBeInTheDocument();
  });

  it("labels a not-scheduled child by the care day, not by the absence the status day reports", () => {
    render(
      <PlannedNowSection
        plannedNow={[
          {
            ...plannedInstance,
            expectedStudentsCount: 1,
            notScheduledStudentsCount: 1,
            rosterPreview: [
              plannedInstance.rosterPreview[0]!,
              {
                ...plannedInstance.rosterPreview[0]!,
                studentId: "student-2",
                studentName: "Jonas Weber",
                // The status-day layer owns this false absence: the child is
                // simply not booked today, so the backend reports "absent"
                // alongside the not_scheduled verdict.
                status: "absent",
                careDayStatus: "not_scheduled",
              },
            ],
          },
        ]}
        isStartingInstance={null}
        onStart={vi.fn()}
      />,
    );

    // Showing "Abwesend" here would contradict the not-scheduled count in the
    // header and the full roster, which groups the row with the other
    // not-scheduled children (#1747).
    expect(screen.getByText("Nicht eingeplant")).toBeInTheDocument();
    expect(screen.queryByText("Abwesend")).toBeNull();
    expect(
      screen.getByText("1 erwartet · 1 heute nicht eingeplant"),
    ).toBeInTheDocument();
  });

  it("keeps a present child expected even when the care plan says otherwise", () => {
    render(
      <PlannedNowSection
        plannedNow={[
          {
            ...plannedInstance,
            rosterPreview: [
              {
                ...plannedInstance.rosterPreview[0]!,
                careDayStatus: "not_scheduled",
                currentlyPresent: true,
                status: "present",
              },
            ],
          },
        ]}
        isStartingInstance={null}
        onStart={vi.fn()}
      />,
    );

    expect(screen.getAllByText("Anwesend").length).toBeGreaterThan(0);
    expect(screen.queryByText("Nicht eingeplant")).toBeNull();
  });

  // The preview label and the Erwartet stat sit on the same card and must
  // agree. Sick, absent and already-present rows are not expected either, so
  // deriving the label from the roster length minus the not-scheduled rows
  // overstates it (#1747 review).
  it("reads the preview label from the backend counts, not the roster length", () => {
    render(
      <PlannedNowSection
        plannedNow={[
          {
            ...plannedInstance,
            expectedStudentsCount: 5,
            notScheduledStudentsCount: 3,
            presentStudentsCount: 0,
            rosterPreview: [
              plannedInstance.rosterPreview[0]!,
              {
                ...plannedInstance.rosterPreview[0]!,
                studentId: "student-2",
                studentName: "Jonas Weber",
                status: "absent",
                substatus: "sick",
              },
              {
                ...plannedInstance.rosterPreview[0]!,
                studentId: "student-3",
                studentName: "Lina Krause",
                careDayStatus: "not_scheduled",
              },
            ],
          },
        ]}
        isStartingInstance={null}
        onStart={vi.fn()}
      />,
    );

    expect(
      screen.getByText("5 erwartet · 3 heute nicht eingeplant"),
    ).toBeInTheDocument();
  });
});
