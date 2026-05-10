import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PlannedNowSection } from "./planned-now-section";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

const plannedInstance: PlannedTimetableInstance = {
  id: "instance-1",
  date: "2026-05-10",
  title: "Hausaufgaben",
  roomId: "room-1",
  startTime: "14:00",
  endTime: "15:00",
  status: "planned",
  expectedStudentsCount: 8,
  presentStudentsCount: 0,
  minutesUntilStart: -5,
  assignedStaffIds: [],
  isOverdue: true,
};

describe("PlannedNowSection", () => {
  it("renders nothing when no planned instances are due", () => {
    const { container } = render(
      <PlannedNowSection
        plannedNow={[]}
        isStartingInstance={null}
        onStart={vi.fn()}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("renders planned instances and starts the selected one", () => {
    const onStart = vi.fn();

    render(
      <PlannedNowSection
        plannedNow={[plannedInstance]}
        isStartingInstance="instance-1"
        onStart={onStart}
      />,
    );

    expect(screen.getByText("Jetzt geplant")).toBeInTheDocument();
    expect(screen.getByText("1 Aktivität")).toBeInTheDocument();
    expect(screen.getByText("Hausaufgaben")).toBeInTheDocument();
    expect(screen.getByText("Überfällig")).toBeInTheDocument();
    expect(screen.getByText("8 erwartet")).toBeInTheDocument();

    const startButton = screen.getByRole("button", { name: "Jetzt starten" });
    expect(startButton).toBeDisabled();

    render(
      <PlannedNowSection
        plannedNow={[plannedInstance]}
        isStartingInstance={null}
        onStart={onStart}
      />,
    );

    fireEvent.click(
      screen.getAllByRole("button", { name: "Jetzt starten" })[1]!,
    );
    expect(onStart).toHaveBeenCalledWith(plannedInstance);
  });

  it("renders plural labels and hides overdue badge for on-time instances", () => {
    render(
      <PlannedNowSection
        plannedNow={[
          { ...plannedInstance, id: "instance-1", isOverdue: false },
          { ...plannedInstance, id: "instance-2", title: "AG Sport" },
        ]}
        isStartingInstance={null}
        onStart={vi.fn()}
      />,
    );

    expect(screen.getByText("2 Aktivitäten")).toBeInTheDocument();
    expect(screen.getByText("AG Sport")).toBeInTheDocument();
    expect(screen.getAllByText("Überfällig")).toHaveLength(1);
  });
});
