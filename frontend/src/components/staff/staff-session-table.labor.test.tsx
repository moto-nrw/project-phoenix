import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { StaffHistorySession } from "~/lib/staff-api";
import type {
  WeeklySummary,
  WorkDayLaborHint,
} from "~/lib/time-tracking-helpers";

import { StaffSessionTable } from "./staff-session-table";

// Hinweise zu Pausen und Höchstarbeitszeit (#4804): die Tabelle zeigt, was
// der Server bewertet hat, und rechnet keine Schwelle selbst.

// Eine Woche Mo 05.01. bis So 11.01.2026, komplett in der Vergangenheit.
const from = new Date(2026, 0, 5);
const to = new Date(2026, 0, 11);
const today = new Date(2026, 5, 15);

function session(date: string, netMinutes: number): StaffHistorySession {
  return {
    id: date.replaceAll("-", ""),
    date,
    net_minutes: netMinutes,
    check_in_time: `${date}T07:00:00Z`,
    check_out_time: `${date}T17:00:00Z`,
    break_minutes: 0,
  };
}

function laborDay(
  date: string,
  overrides: Partial<WorkDayLaborHint> = {},
): WorkDayLaborHint {
  return {
    date,
    netMinutes: 480,
    breakMinutes: 30,
    countedBreakMinutes: 30,
    requiredBreakMinutes: 30,
    isBreakTooShort: false,
    isOverDailyMax: false,
    isRunning: false,
    overDailyMaxAt: null,
    breakTooShortAt: null,
    breakTooShortRequiredMinutes: 0,
    ...overrides,
  };
}

function week(overrides: Partial<WeeklySummary> = {}): WeeklySummary {
  return {
    weekNumber: 2,
    year: 2026,
    totalNetMinutes: 2400,
    targetMinutes: null,
    deltaMinutes: null,
    sessionCount: 5,
    isOverWeeklyMax: false,
    ...overrides,
  };
}

function renderTable(props: {
  sessions: readonly StaffHistorySession[];
  laborDays?: readonly WorkDayLaborHint[];
  weeklySummaries?: readonly WeeklySummary[];
  from?: Date;
  to?: Date;
}) {
  return render(
    <StaffSessionTable
      staffId="1"
      from={props.from ?? from}
      to={props.to ?? to}
      sessions={props.sessions}
      schedule={null}
      accountStartDate=""
      accountStartDatePending={false}
      accountStartDateError={false}
      today={today}
      isAdminView
      laborDays={props.laborDays}
      weeklySummaries={props.weeklySummaries}
    />,
  );
}

function rowOf(text: string): HTMLElement {
  const row = screen.getByText(text).closest("tr");
  if (!row) throw new Error(`no row for ${text}`);
  return row;
}

describe("StaffSessionTable Pausen und Höchstarbeitszeit (#4804)", () => {
  it("markiert einen Tag mit zu kurzer Pause mit den Minuten vom Server", () => {
    renderTable({
      sessions: [session("2026-01-05", 370), session("2026-01-06", 390)],
      laborDays: [
        laborDay("2026-01-05", {
          netMinutes: 370,
          countedBreakMinutes: 15,
          isBreakTooShort: true,
        }),
        laborDay("2026-01-06"),
      ],
    });

    const marked = rowOf("05.01.");
    expect(within(marked).getByText("Pause zu kurz")).toBeInTheDocument();
    expect(
      within(marked).getByText("30 Min. nötig, 15 Min. gemacht"),
    ).toBeInTheDocument();
    const unmarked = rowOf("06.01.");
    expect(within(unmarked).getByText("06.01.")).toBeInTheDocument();
    expect(within(unmarked).queryByText(/Pause zu kurz/)).toBeNull();
  });

  it("markiert einen Tag mit mehr als 10 Stunden", () => {
    renderTable({
      sessions: [session("2026-01-05", 615)],
      laborDays: [laborDay("2026-01-05", { isOverDailyMax: true })],
    });

    expect(
      within(rowOf("05.01.")).getByText("Mehr als 10 Std. gearbeitet"),
    ).toBeInTheDocument();
  });

  it("schließt die Woche mit ihrer Summe ab und markiert mehr als 48 Stunden", () => {
    renderTable({
      sessions: [session("2026-01-05", 588)],
      weeklySummaries: [
        week({ totalNetMinutes: 49 * 60, isOverWeeklyMax: true }),
      ],
    });

    const weekRow = rowOf("Summe KW 2");
    expect(within(weekRow).getByText("49h")).toBeInTheDocument();
    expect(
      within(weekRow).getByText("Mehr als 48 Std. in der Woche"),
    ).toBeInTheDocument();
  });

  it("markiert eine Woche bis 48 Stunden nicht", () => {
    renderTable({
      sessions: [session("2026-01-05", 480)],
      weeklySummaries: [week({ totalNetMinutes: 48 * 60 })],
    });

    const weekRow = rowOf("Summe KW 2");
    expect(within(weekRow).getByText("48h")).toBeInTheDocument();
    expect(within(weekRow).queryByText(/Mehr als 48/)).toBeNull();
  });

  it("nennt eine Woche über den Rand der Ansicht als ganze Woche", () => {
    // Monatsblick Januar: die erste Woche beginnt am 29.12.2025.
    renderTable({
      sessions: [session("2026-01-02", 480)],
      weeklySummaries: [week({ weekNumber: 1, totalNetMinutes: 2880 })],
      from: new Date(2026, 0, 1),
      to: new Date(2026, 0, 31),
    });

    expect(screen.getByText("Summe KW 1 (ganze Woche)")).toBeInTheDocument();
  });

  it("zeigt ohne Wochensummen keine Summenzeile", () => {
    renderTable({ sessions: [session("2026-01-05", 480)] });

    expect(screen.getByText("05.01.")).toBeInTheDocument();
    expect(screen.queryByText(/Summe KW/)).toBeNull();
  });
});
