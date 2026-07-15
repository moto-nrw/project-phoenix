import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { StaffSchedule } from "~/lib/staff-api";

import { StaffSessionTable } from "./staff-session-table";

// Mo–Fr 8h nach dem HEUTE gültigen Plan. Genau dieser Plan darf bei einem
// fehlgeschlagenen Targets-Fetch NICHT auf vergangene Tage angewendet werden
// (#1842): wer seine Stunden geändert hat, bekäme sonst ein erfundenes
// historisches Soll, das der Monatskarte widerspricht.
const schedule: StaffSchedule = {
  mode: "custom",
  model: null,
  rotationLength: 1,
  rotationAnchorDate: "2026-01-05",
  entries: [0, 1, 2, 3, 4].map((dayOfWeek) => ({
    weekIndex: 0,
    dayOfWeek,
    targetMinutes: 480,
  })),
  weeklyTotals: [2400],
  validFrom: "2026-01-05",
};

// Eine Woche Mo–So, komplett in der Vergangenheit.
const from = new Date(2026, 0, 5);
const to = new Date(2026, 0, 11);
const today = new Date(2026, 5, 15);

function renderTable(props: {
  dailyTargets?: ReadonlyMap<string, number>;
  dailyTargetsError?: boolean;
}) {
  return render(
    <StaffSessionTable
      staffId="1"
      from={from}
      to={to}
      sessions={[]}
      schedule={schedule}
      today={today}
      isAdminView
      {...props}
    />,
  );
}

describe("StaffSessionTable Soll-Auflösung", () => {
  it("nutzt die servergelieferten Targets, wenn vorhanden", () => {
    renderTable({ dailyTargets: new Map([["2026-01-05", 300]]) });

    // 5h aus den Targets, nicht 8h aus dem aktuellen Plan.
    expect(screen.getAllByText("5h").length).toBeGreaterThan(0);
  });

  it("fällt auf den Plan zurück, solange die Targets laden", () => {
    renderTable({});

    // Mo–Fr aus dem Plan, kein Fehlerhinweis.
    expect(screen.getAllByText("8h").length).toBe(5);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("zeigt bei fehlgeschlagenem Targets-Fetch kein Soll aus dem aktuellen Plan", () => {
    renderTable({ dailyTargetsError: true });

    // Der aktuelle Plan darf nicht als historisches Soll auftauchen.
    expect(screen.queryByText("8h")).not.toBeInTheDocument();
    // Die Tabelle zeigt nur Mo–Fr.
    expect(screen.getAllByText("?").length).toBe(5);
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("bevorzugt geladene Targets auch dann, wenn ein Fehler gemeldet ist", () => {
    renderTable({
      dailyTargets: new Map([["2026-01-05", 300]]),
      dailyTargetsError: true,
    });

    expect(screen.getAllByText("5h").length).toBeGreaterThan(0);
    // Nur die Tage ohne aufgelöstes Target bleiben ungelöst.
    expect(screen.getAllByText("?").length).toBe(4);
  });
});
