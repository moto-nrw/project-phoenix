import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type {
  StaffAbsenceRow,
  StaffHistorySession,
  StaffSchedule,
} from "~/lib/staff-api";

import { StaffSessionTable } from "./staff-session-table";

// Mo–Fr 8h nach dem heute gültigen Plan — nur damit die Zeilen sichtbar sind.
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

const mondaySession: StaffHistorySession = {
  id: 41,
  date: "2026-01-05",
  net_minutes: 180,
  check_in_time: "2026-01-05T09:00:00Z",
  check_out_time: "2026-01-05T12:00:00Z",
  break_minutes: 0,
};

const mondaySickAbsence: StaffAbsenceRow = {
  id: 7,
  staff_id: 1,
  absence_type: "sick",
  date_start: "2026-01-05",
  date_end: "2026-01-05",
  half_day: true,
  note: "",
  status: "approved",
};

const mondayFullDayAbsence: StaffAbsenceRow = {
  ...mondaySickAbsence,
  half_day: false,
};

function renderTable(props: {
  sessions?: readonly StaffHistorySession[];
  absences?: readonly StaffAbsenceRow[];
  absencesPending?: boolean;
}) {
  return render(
    <StaffSessionTable
      staffId="1"
      from={from}
      to={to}
      sessions={props.sessions ?? []}
      absences={props.absences}
      absencesPending={props.absencesPending}
      schedule={schedule}
      accountStartDate=""
      accountStartDatePending={false}
      accountStartDateError={false}
      today={today}
      isAdminView
    />,
  );
}

// Der Stift darf auf Tagen mit Abwesenheit nicht fehlen (#2361): eine halbe
// Krankmeldung schließt Arbeitszeit am selben Tag nicht aus, und Nachtragen
// muss auch dort erreichbar sein.
describe("StaffSessionTable Stift-Verfügbarkeit", () => {
  it("zeigt den Nachtragen-Stift auch auf einem Tag mit Abwesenheit ohne Buchung", () => {
    renderTable({ absences: [mondaySickAbsence] });

    const row = screen.getByText("05.01.").closest("tr");
    expect(row).not.toBeNull();
    expect(
      within(row!).getByRole("button", { name: "Eintrag nachtragen" }),
    ).toBeInTheDocument();
  });

  it("zeigt den Bearbeiten-Stift auf einem Tag mit Buchung und Abwesenheit", () => {
    renderTable({
      sessions: [mondaySession],
      absences: [mondaySickAbsence],
    });

    const row = screen.getByText("05.01.").closest("tr");
    expect(row).not.toBeNull();
    expect(
      within(row!).getByRole("button", { name: "Eintrag bearbeiten" }),
    ).toBeInTheDocument();
  });

  it.each([
    ["sick", "reported"],
    ["sick", "approved"],
    ["vacation", "reported"],
    ["vacation", "approved"],
    ["training", "reported"],
    ["training", "approved"],
  ])(
    "zeigt keinen Nachtragen-Stift bei ganztägiger %s-Abwesenheit mit Status %s",
    (absenceType, status) => {
      renderTable({
        absences: [
          { ...mondayFullDayAbsence, absence_type: absenceType, status },
        ],
      });

      const row = screen.getByText("05.01.").closest("tr");
      expect(row).not.toBeNull();
      expect(
        within(row!).queryByRole("button", { name: "Eintrag nachtragen" }),
      ).not.toBeInTheDocument();
    },
  );

  it("zeigt keinen Nachtragen-Stift, solange Abwesenheiten laden", () => {
    renderTable({ absencesPending: true });

    const row = screen.getByText("05.01.").closest("tr");
    expect(row).not.toBeNull();
    expect(
      within(row!).queryByRole("button", { name: "Eintrag nachtragen" }),
    ).not.toBeInTheDocument();
  });
});
