import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { GapInstance } from "~/lib/timetable-types";

import { GapJumpList } from "./gap-jump-list";

const gap: GapInstance = {
  instanceId: "42",
  date: "2026-05-04",
  title: "Mensa",
  startTime: "12:00",
  endTime: "13:00",
  roomId: "3",
  status: "planned",
  assignedStaffCount: 1,
  absentStaffCount: 1,
  presentStaffCount: 1,
  plannedStaffCount: 3,
};

describe("GapJumpList", () => {
  it("shows the coverage pair when the response includes a staffing target", () => {
    render(<GapJumpList gaps={[gap]} onJump={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: /1 Lücke/ }));

    expect(
      screen.getByLabelText("1 von 3 Positionen besetzt, 2 offen"),
    ).toHaveTextContent("1/3");
  });

  it("omits the coverage pair for an unstaffed gap without planned positions", () => {
    render(
      <GapJumpList
        gaps={[
          {
            ...gap,
            assignedStaffCount: 0,
            absentStaffCount: 0,
            presentStaffCount: 0,
            plannedStaffCount: 0,
          },
        ]}
        onJump={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /1 Lücke/ }));

    expect(screen.getByLabelText("Offene Lücke")).toBeInTheDocument();
    expect(screen.queryByText("0/0")).not.toBeInTheDocument();
  });
});
