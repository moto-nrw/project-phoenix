import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type {
  StaffAbsenceRow,
  StaffHistorySession,
  StaffSchedule,
} from "~/lib/staff-api";
import type { StaffShift } from "~/lib/shift-helpers";

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
  dailyTargetsPending?: boolean;
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

  it("zeigt während des Ladens kein Soll aus dem aktuellen Plan", () => {
    // Die Fetches laufen mit keepPreviousData: nach einem Zeitraumwechsel hält
    // `dailyTargets` noch die Keys des VORHERIGEN Zeitraums, die neuen Tage
    // fehlen. Der Plan als Lückenfüller zeigte dort kurzzeitig ein falsches
    // historisches Soll, bis die Antwort eintraf (#1842).
    renderTable({ dailyTargetsPending: true });

    expect(screen.queryByText("8h")).not.toBeInTheDocument();
    expect(screen.getAllByText("…").length).toBe(5);
    // Laden ist kein Fehler — kein Warnhinweis.
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("behält beim Zeitraumwechsel die bereits aufgelösten Tage", () => {
    // Ein überlappender Tag aus dem vorherigen Fetch bleibt gültig: dasselbe
    // Datum liefert für dieselbe Person immer dasselbe Soll. Nur die noch
    // nicht abgedeckten Tage bleiben ungelöst.
    renderTable({
      dailyTargets: new Map([["2026-01-05", 300]]),
      dailyTargetsPending: true,
    });

    expect(screen.getAllByText("5h").length).toBeGreaterThan(0);
    expect(screen.getAllByText("…").length).toBe(4);
    expect(screen.queryByText("8h")).not.toBeInTheDocument();
  });

  it("nutzt ohne Lade- und Fehlersignal den Plan als Fallback", () => {
    // Der unsignalisierte Default (kein Caller im Produktivcode): ohne jede
    // Angabe bleibt der Plan die einzige Quelle.
    renderTable({});

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

// #1967: Sa/So waren hart aus der Tagesansicht gefiltert, während Monatskarte
// und KPI-Karten sie serverseitig mitzählten. Die Summe der sichtbaren Zeilen
// ergab dann nicht mehr das ausgewiesene Ist.
describe("StaffSessionTable Wochenendtage", () => {
  // Sa, 10.01.2026 — im Zeitraum from/to und in der Vergangenheit.
  const saturday = "2026-01-10";

  const weekendSession: StaffHistorySession = {
    id: 42,
    date: saturday,
    net_minutes: 180,
    check_in_time: `${saturday}T09:00:00Z`,
    check_out_time: `${saturday}T12:00:00Z`,
    break_minutes: 0,
  };

  function renderWeekend(props: {
    sessions?: readonly StaffHistorySession[];
    plannedShifts?: readonly StaffShift[];
    absences?: readonly StaffAbsenceRow[];
  }) {
    return render(
      <StaffSessionTable
        staffId="1"
        from={from}
        to={to}
        sessions={props.sessions ?? []}
        absences={props.absences}
        plannedShifts={props.plannedShifts}
        schedule={schedule}
        dailyTargets={
          new Map([
            ["2026-01-05", 480],
            ["2026-01-06", 480],
            ["2026-01-07", 480],
            ["2026-01-08", 480],
            ["2026-01-09", 480],
            // Der Server liefert Wochenenden mit Soll 0 mit.
            ["2026-01-10", 0],
            ["2026-01-11", 0],
          ])
        }
        today={today}
        isAdminView
      />,
    );
  }

  it("blendet leere Wochenenden weiter aus", () => {
    renderWeekend({});

    // 10.01. = Sa, 11.01. = So — beide Datumszellen fehlen.
    expect(screen.queryByText("10.01.")).not.toBeInTheDocument();
    expect(screen.queryByText("11.01.")).not.toBeInTheDocument();
  });

  it("zeigt einen Samstag mit Session als Zeile", () => {
    renderWeekend({ sessions: [weekendSession] });

    expect(screen.getByText("10.01.")).toBeInTheDocument();
    // Der Sonntag bleibt leer und damit unsichtbar.
    expect(screen.queryByText("11.01.")).not.toBeInTheDocument();
  });

  it("zählt das Ist eines Wochenendtags voll als Plus", () => {
    // Soll ist 0, die Monatskarte rechnet actual - targetToDate. Ein "–" im
    // Saldo machte die Monatskarte aus der Tabelle unerklärbar.
    renderWeekend({ sessions: [weekendSession] });

    expect(screen.getByText("+3h")).toBeInTheDocument();
  });

  it("zeigt einen Wochenendtag mit geplanter Schicht", () => {
    renderWeekend({
      plannedShifts: [
        {
          id: "1",
          staffId: "1",
          date: saturday,
          startTime: "09:00",
          endTime: "12:00",
          breakMinutes: 0,
          shiftTypeId: null,
          shiftTypeName: null,
          shiftTypeColor: null,
          notes: "",
          seriesId: null,
          detached: false,
          cancelled: false,
          changeReason: null,
          originShiftId: null,
        },
      ],
    });

    expect(screen.getByText("10.01.")).toBeInTheDocument();
  });
});
