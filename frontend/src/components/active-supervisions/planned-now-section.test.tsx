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
    expect(screen.getByText("Primär")).toBeInTheDocument();
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
});
