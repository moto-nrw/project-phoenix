import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { StaffHistorySession, StaffSchedule } from "~/lib/staff-api";

import { StaffSessionTable } from "./staff-session-table";

// Mehrere Arbeitsblöcke pro Tag (#2402): Homeoffice-Vormittag, OGS-Nachmittag.
// Die Tageszeile aggregiert, die Block-Zeilen tragen die Details.

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

// Nur der Montag, komplett in der Vergangenheit.
const from = new Date(2026, 0, 5);
const to = new Date(2026, 0, 5);
const today = new Date(2026, 5, 15);

const morningHomeOffice: StaffHistorySession = {
  id: 41,
  date: "2026-01-05",
  status: "home_office",
  source: "app",
  net_minutes: 240,
  check_in_time: "2026-01-05T08:00:00+01:00",
  check_out_time: "2026-01-05T12:00:00+01:00",
  break_minutes: 0,
};

const afternoonOgs: StaffHistorySession = {
  id: 42,
  date: "2026-01-05",
  status: "present",
  source: "nfc",
  net_minutes: 130,
  check_in_time: "2026-01-05T13:30:00+01:00",
  check_out_time: "2026-01-05T16:00:00+01:00",
  break_minutes: 20,
};

function renderTable(props?: {
  sessions?: readonly StaffHistorySession[];
  onEditDay?: (
    date: Date,
    session: StaffHistorySession | null,
    absence: unknown,
  ) => void;
}) {
  return render(
    <StaffSessionTable
      staffId="1"
      from={from}
      to={to}
      // Absichtlich verdreht übergeben — die Tabelle sortiert nach Check-in.
      sessions={props?.sessions ?? [afternoonOgs, morningHomeOffice]}
      schedule={schedule}
      dailyTargets={new Map([["2026-01-05", 480]])}
      accountStartDate=""
      accountStartDatePending={false}
      accountStartDateError={false}
      today={today}
      isAdminView
      onEditDay={props?.onEditDay}
    />,
  );
}

describe("StaffSessionTable Arbeitsblöcke (#2402)", () => {
  it("aggregiert die Tageszeile über alle Blöcke", () => {
    renderTable();

    // Check-in = erster Block (Tageszeile + Block-1-Zeile), Check-out =
    // letzter Block.
    expect(screen.getAllByText("08:00").length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText("16:00").length).toBeGreaterThan(0);
    // Ist = 240 + 130 = 370min = 6h 10min, Pause = 20min (nur Block 2).
    expect(screen.getAllByText("6h 10min").length).toBeGreaterThan(0);
    // Statuszelle der Tageszeile trägt den Block-Zähler.
    expect(screen.getByText("2 Blöcke")).toBeInTheDocument();
  });

  it("paart Status und Quelle nur in den Block-Zeilen, nie auf der Tageszeile", () => {
    renderTable();

    // Die Tageszeile stapelt KEINE Status-/Quelle-Badges nebeneinander —
    // das las sich als "OGS · App", obwohl der OGS-Block per NFC kam.
    // Jeder Arbeitsort und jede Quelle erscheint genau einmal, nämlich in
    // der Block-Zeile, zu der sie gehören.
    expect(screen.getAllByText("OGS")).toHaveLength(1);
    expect(screen.getAllByText("Homeoffice")).toHaveLength(1);
    expect(screen.getAllByText("NFC")).toHaveLength(1);
    expect(screen.getAllByText("App")).toHaveLength(1);

    // Homeoffice-Block kam per App, OGS-Block per NFC: die Badges sitzen in
    // derselben Zeile wie ihr Block.
    const homeOfficeRow = screen.getByText("Homeoffice").closest("tr");
    const ogsRow = screen.getByText("OGS").closest("tr");
    expect(homeOfficeRow).toContainElement(screen.getByText("App"));
    expect(ogsRow).toContainElement(screen.getByText("NFC"));
  });

  it("zeigt die Quelle auf der Tageszeile nur bei einheitlichem Kanal", () => {
    renderTable({
      sessions: [morningHomeOffice, { ...afternoonOgs, source: "app" }],
    });

    // Beide Blöcke per App → die Tageszeile darf das Badge zeigen
    // (Tageszeile + zwei Block-Zeilen = 3).
    expect(screen.getAllByText("App")).toHaveLength(3);
  });

  it("listet jeden Block mit eigenen Zeiten als Unterzeile", () => {
    renderTable();

    expect(screen.getByText("Block 1")).toBeInTheDocument();
    expect(screen.getByText("Block 2")).toBeInTheDocument();
    // Blockzeiten in Check-in-Reihenfolge, trotz verdrehter Eingabe.
    expect(screen.getByText("12:00")).toBeInTheDocument();
    expect(screen.getByText("13:30")).toBeInTheDocument();
  });

  it("öffnet die Bearbeitung für genau den angeklickten Block", () => {
    const onEditDay = vi.fn();
    renderTable({ onEditDay });

    fireEvent.click(screen.getByLabelText("Block 2 bearbeiten"));

    expect(onEditDay).toHaveBeenCalledTimes(1);
    const [, session] = onEditDay.mock.calls[0] as [
      Date,
      StaffHistorySession | null,
      unknown,
    ];
    expect(session?.id).toBe(42);
  });

  it("bietet auf einem Mehrblock-Tag das Nachtragen eines weiteren Blocks an", () => {
    const onEditDay = vi.fn();
    renderTable({ onEditDay });

    fireEvent.click(screen.getByLabelText("Block nachtragen"));

    expect(onEditDay).toHaveBeenCalledTimes(1);
    const [, session] = onEditDay.mock.calls[0] as [
      Date,
      StaffHistorySession | null,
      unknown,
    ];
    expect(session).toBeNull();
  });

  it("Ein-Block-Tage rendern unverändert ohne Unterzeilen", () => {
    renderTable({ sessions: [morningHomeOffice] });

    expect(screen.queryByText("Block 1")).not.toBeInTheDocument();
    expect(screen.queryByText("2 Blöcke")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Eintrag bearbeiten")).toBeInTheDocument();
  });

  it("markiert den Tag als eingestempelt, wenn ein Block offen ist", () => {
    renderTable({
      sessions: [morningHomeOffice, { ...afternoonOgs, check_out_time: null }],
    });

    expect(screen.getAllByText("eingestempelt").length).toBeGreaterThan(0);
  });
});
