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
});
