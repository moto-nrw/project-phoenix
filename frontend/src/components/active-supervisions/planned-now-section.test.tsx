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
      screen.getByText("Betreuung starten und Raum aktivieren"),
    ).toBeInTheDocument();
    expect(screen.getByText("1 geplant")).toBeInTheDocument();
    expect(screen.getByText("8 Kinder")).toBeInTheDocument();
    expect(screen.getByText("Hausaufgaben")).toBeInTheDocument();
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

    fireEvent.click(screen.getByRole("button", { name: /Jetzt starten/i }));
    expect(onStart).toHaveBeenCalledWith(plannedInstance);
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
    expect(screen.getByText("AG Sport")).toBeInTheDocument();
    expect(screen.getAllByText("Startet gleich")).toHaveLength(1);
    expect(screen.getByText("Vertretung")).toBeInTheDocument();
  });
});
