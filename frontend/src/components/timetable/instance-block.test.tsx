import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import { InstanceBlock } from "./instance-block";
import type { EnrichedInstance } from "~/lib/timetable-types";

/**
 * InstanceBlock renders internally via the kit primitive PlanBlock. These
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
  extra: { isGap?: boolean; staffNames?: ReadonlyMap<string, string> } = {},
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
        staffNames={extra.staffNames}
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
    expect(screen.getByText("2")).toHaveStyle({ color: "#DC2626" });
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
    expect(icon).toHaveClass("text-moto-orange");
  });

  it("lets cancelled beat gap: a cancelled gap block shows no gap icon", () => {
    renderBlock(makeInstance({ status: "cancelled" }), { isGap: true });

    expect(screen.queryByLabelText("Offene Lücke")).not.toBeInTheDocument();
    expect(screen.getByText("Mensa")).toHaveClass("line-through");
  });

  // #3817: Raum, Gruppe und Fachkräfte stehen im Block selbst.
  describe("Raum, Gruppe und Fachkräfte im Block", () => {
    const names = new Map([
      ["1", "Anna Kowalski"],
      ["2", "Ben Müller"],
    ]);
    const staffed = (overrides: Partial<EnrichedInstance> = {}) =>
      makeInstance({
        title: "Hausaufgaben",
        roomName: "Raum 104",
        groupName: "Sonne",
        absentStaffCount: 1,
        staff: [
          {
            staffId: "1",
            isPrimary: true,
            isAbsent: true,
            isSubstitute: false,
          },
          {
            staffId: "2",
            isPrimary: false,
            isAbsent: false,
            isSubstitute: true,
          },
        ],
        ...overrides,
      });

    it("zeigt in einem hohen Block Raum, Gruppe und Namen in zwei Zeilen", () => {
      renderBlock(staffed(), { staffNames: names }, 130);

      expect(screen.getByText("Raum 104 · Sonne")).toBeInTheDocument();
      expect(screen.getByText("Ben M. (Ersatz)")).toBeInTheDocument();
      expect(screen.getByText("Anna K.", { exact: false })).toHaveClass(
        "line-through",
      );
      // Die Namen zeigen Abwesenheit und Ersatz schon; die Sammelzeile entfällt.
      expect(screen.queryByText("1 abwesend")).not.toBeInTheDocument();
    });

    it("fasst Ort und Namen in einem 60-Minuten-Block zusammen und behält die Besetzung", () => {
      renderBlock(
        staffed({ assignedStaffCount: 1, requiredStaffCount: 2 }),
        { staffNames: names },
        90,
      );

      expect(screen.getByText("Raum 104 · Sonne ·")).toBeInTheDocument();
      expect(screen.getByText("Ben M. (Ersatz)")).toBeInTheDocument();
      // Die Besetzung folgt direkt auf die Infozeile.
      expect(screen.getByText("/2")).toBeInTheDocument();
    });

    it("trägt den vollständigen Inhalt im Tooltip und im Screenreader-Namen", () => {
      renderBlock(staffed(), { staffNames: names });

      const button = screen.getByRole("button");
      expect(button).toHaveAttribute(
        "title",
        [
          "Hausaufgaben, 12:00 – 13:00",
          "Raum: Raum 104",
          "Gruppe: Sonne",
          "Fachkräfte: Anna Kowalski (abwesend), Ben Müller (Ersatz)",
        ].join("\n"),
      );
      expect(button).toHaveAccessibleName(
        /Gruppe: Sonne, Fachkräfte: Anna Kowalski \(abwesend\), Ben Müller \(Ersatz\)/,
      );
    });

    it("zeigt in einem kurzen Block eine Infozeile unter Zeit und Titel", () => {
      renderBlock(staffed(), { staffNames: names }, 45);

      expect(screen.getByText("Raum 104 · Sonne ·")).toBeInTheDocument();
      expect(screen.getByText("Ben M. (Ersatz)")).toBeInTheDocument();
      expect(screen.getByText("Hausaufgaben")).toBeInTheDocument();
    });

    it("zeigt in einem sehr kurzen Block nur Zeit und Titel", () => {
      renderBlock(staffed(), { staffNames: names }, 28);

      expect(screen.queryByText(/Raum 104/)).not.toBeInTheDocument();
      expect(screen.getByRole("button")).toHaveAttribute(
        "title",
        expect.stringContaining("Raum: Raum 104"),
      );
    });

    it("behält ohne bekannte Namen die Sammelzeile für Abwesenheit und Ersatz", () => {
      renderBlock(staffed());

      expect(screen.getByText("Raum 104 · Sonne")).toBeInTheDocument();
      expect(screen.getByText("1 abwesend")).toBeInTheDocument();
      expect(screen.getByText("Ersatz")).toBeInTheDocument();
    });

    it("zeigt bei einem abgesagten Block weder Ort noch Namen", () => {
      renderBlock(staffed({ status: "cancelled" }), { staffNames: names });

      expect(screen.queryByText(/Raum 104/)).not.toBeInTheDocument();
      expect(screen.queryByText(/Ben M\./)).not.toBeInTheDocument();
    });
  });

  // #3634: a running block compares the children still there with the
  // activity's limit, and names an overbooked block in text.
  describe("Teilnehmergrenze eines laufenden Blocks", () => {
    const live = (current: number, limit: number | null, present = current) =>
      makeInstance({
        status: "active",
        isLive: true,
        presentStudentsCount: present,
        currentStudentsCount: current,
        occupancy:
          limit === null
            ? null
            : { participantLimit: limit, currentStudentsCount: current },
      });

    it("zeigt Anwesende gegen die Grenze unter der Grenze", () => {
      renderBlock(live(40, 45));
      expect(screen.getByText(/40 \/ 45 anwesend/)).toBeInTheDocument();
      expect(screen.queryByText("Überbucht")).not.toBeInTheDocument();
    });

    it("markiert einen vollen Block nicht als überbucht", () => {
      renderBlock(live(45, 45));
      expect(screen.getByText(/45 \/ 45 anwesend/)).toBeInTheDocument();
      expect(screen.queryByText("Überbucht")).not.toBeInTheDocument();
    });

    it("nennt einen überbuchten Block mit Text", () => {
      renderBlock(live(66, 45));
      expect(screen.getByText("Überbucht")).toBeInTheDocument();
      expect(screen.getByText(/66 \/ 45 anwesend/)).toBeInTheDocument();
    });

    it("zählt Kinder, die schon gegangen sind, nicht gegen die Grenze", () => {
      renderBlock(live(43, 45, 47));
      expect(screen.getByText(/43 \/ 45 anwesend/)).toBeInTheDocument();
      expect(screen.queryByText("Überbucht")).not.toBeInTheDocument();
    });

    it("zeigt ohne Grenze nur die Anwesenden", () => {
      renderBlock(live(66, null));
      expect(screen.getByText(/66 anwesend/)).toBeInTheDocument();
      expect(screen.queryByText(/\/ 45/)).not.toBeInTheDocument();
      expect(screen.queryByText("Überbucht")).not.toBeInTheDocument();
    });

    // #3921: auch ohne Grenze zählt nur, wer noch da ist.
    it("zählt ohne Grenze Kinder, die gegangen sind, nicht als anwesend", () => {
      renderBlock(live(43, null, 47));
      expect(screen.getByText(/43 anwesend/)).toBeInTheDocument();
      expect(screen.queryByText(/47 anwesend/)).not.toBeInTheDocument();
    });
  });
});
