import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import { InstanceBlock } from "./instance-block";
import type { EnrichedInstance } from "~/lib/timetable-types";

/**
 * InstanceBlock renders internally via the kit primitive PlanBlock
 * (docs/planung-redesign/docs/06-betreuungsplan.md Abschnitt 2.2/5.1). These
 * tests pin the data-to-PlanBlock mapping: planning-track edge color, the footer
 * CoverageIndicator numbers (Kriterium 6), cancelled rendering, the
 * acknowledged gray-with-note state, and the single-status-icon priority
 * cancelled > offene Lücke.
 */

function makeInstance(
  overrides: Partial<EnrichedInstance> = {},
): EnrichedInstance {
  return {
    id: "42",
    date: "2026-05-04",
    startTime: "12:00",
    endTime: "13:00",
    title: "Mensa",
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityType: "activity", // -> getActivityColor #83CD2D
    roomId: "3",
    roomName: "Mensa",
    staff: [],
    students: [],
    studentIds: [],
    staffCount: 1,
    absentStaffCount: 0,
    expectedStudentsCount: 0,
    notScheduledStudentsCount: 0,
    presentStudentsCount: 0,
    requiredStaffCount: 3,
    assignedStaffCount: 2,
    conflictWarnings: [],
    ...overrides,
  };
}

function renderBlock(
  instance: EnrichedInstance,
  extra: { isGap?: boolean } = {},
  height = 90,
) {
  return render(
    <div className="relative h-96">
      <InstanceBlock
        instance={instance}
        top={20}
        height={height}
        left="0%"
        width="100%"
        isSelected={false}
        onClick={vi.fn()}
        isGap={extra.isGap}
      />
    </div>,
  );
}

describe("InstanceBlock -> PlanBlock mapping", () => {
  it("renders the neutral edge when no planning track is assigned", () => {
    renderBlock(makeInstance());

    expect(screen.getByRole("button")).toHaveStyle({
      borderLeft: "3px solid #D1D5DB",
    });
  });

  it("shows the Besetzung CoverageIndicator (assigned/required) in the footer", () => {
    renderBlock(makeInstance({ assignedStaffCount: 2, requiredStaffCount: 3 }));

    // Split spans, "2" (Ist) then "/3" (Soll) — the Ist is red on shortfall.
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByText("/3")).toBeInTheDocument();
    expect(screen.getByText("2")).toHaveStyle({ color: "#FF3130" });
  });

  it("renders cancelled instances via the PlanBlock cancelled recipe", () => {
    renderBlock(makeInstance({ status: "cancelled" }));

    const label = screen.getByText("Mensa");
    expect(label).toHaveClass("line-through", "text-gray-400");
    // Cancelled flattens the edge to neutral gray and drops any coverage.
    expect(screen.getByRole("button")).toHaveStyle({
      borderLeft: "3px solid #9CA3AF",
    });
    expect(screen.queryByText("/3")).not.toBeInTheDocument();
  });

  it("renders understaffedAck as a gray block with the bewusst-unbesetzt note", () => {
    renderBlock(makeInstance({ understaffedAck: true }));

    expect(screen.getByText("bewusst unbesetzt")).toBeInTheDocument();
    // Gray edge, no tint (deliberately-unstaffed reads fully neutral).
    const button = screen.getByRole("button");
    expect(button).toHaveStyle({ borderLeft: "3px solid #6B7280" });
    expect(button.style.backgroundColor).toBe("");
    // acknowledged dot color (the only aria-hidden element in this render)
    const dot = document.querySelector('[aria-hidden="true"]');
    expect(dot).toHaveStyle({ backgroundColor: "#6B7280" });
  });

  it("keeps acknowledged understaffing visible whenever the footer is hidden", () => {
    // 60px liegt über der Zwei-Zeilen-Kurzblockschwelle, aber unter der
    // Fußzeilenschwelle: auch dieser Zwischenbereich braucht den Fallback.
    renderBlock(makeInstance({ understaffedAck: true }), {}, 60);

    expect(screen.getByLabelText("Bewusst unbesetzt")).toBeInTheDocument();
    expect(screen.getByRole("button")).toHaveAccessibleName(
      /bewusst unbesetzt/i,
    );
  });

  it("keeps compact signals when the footer is hidden", () => {
    renderBlock(
      makeInstance({
        status: "active",
        assignedStaffCount: 1,
        requiredStaffCount: 2,
        absentStaffCount: 1,
        staff: [
          {
            staffId: "13",
            isPrimary: false,
            isAbsent: false,
            isSubstitute: true,
          },
        ],
      }),
      {},
      60,
    );

    expect(screen.getByRole("button")).toHaveAccessibleName(
      /1 von 2 Positionen besetzt, läuft, 1 abwesend, Ersatz/i,
    );
  });

  it("shows the single #F78C10 gap icon when the block is an open gap", () => {
    renderBlock(makeInstance(), { isGap: true });

    const icon = screen.getByLabelText("Offene Lücke");
    expect(icon).toHaveClass("text-[#F78C10]");
  });

  it("lets cancelled beat gap: a cancelled gap block shows no gap icon", () => {
    renderBlock(makeInstance({ status: "cancelled" }), { isGap: true });

    expect(screen.queryByLabelText("Offene Lücke")).not.toBeInTheDocument();
    expect(screen.getByText("Mensa")).toHaveClass("line-through");
  });
});
