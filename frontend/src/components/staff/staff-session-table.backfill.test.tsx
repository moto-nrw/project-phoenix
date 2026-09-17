import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type {
  StaffAbsenceRow,
  StaffHistorySession,
  StaffSchedule,
} from "~/lib/staff-api";

import { StaffSessionTable } from "./staff-session-table";

// „Abwesenheit nachtragen" (#3258): ein vergangener Tag ohne Eintrag bekommt
// statt Arbeitszeit eine Abwesenheit.
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

const from = new Date(2026, 0, 5);
const to = new Date(2026, 0, 11);
const today = new Date(2026, 5, 15);

const mondaySession: StaffHistorySession = {
  id: "41",
  date: "2026-01-05",
  net_minutes: 180,
  check_in_time: "2026-01-05T09:00:00Z",
  check_out_time: "2026-01-05T12:00:00Z",
  break_minutes: 0,
};

const mondayHalfSick: StaffAbsenceRow = {
  id: 7,
  staff_id: 1,
  absence_type: "sick",
  date_start: "2026-01-05",
  date_end: "2026-01-05",
  half_day: true,
  note: "",
  status: "reported",
};

function renderTable(props: {
  sessions?: readonly StaffHistorySession[];
  absences?: readonly StaffAbsenceRow[];
  onBackfillAbsence?: (date: Date) => void;
}) {
  return render(
    <StaffSessionTable
      staffId="1"
      from={from}
      to={to}
      sessions={props.sessions ?? []}
      absences={props.absences ?? []}
      schedule={schedule}
      accountStartDate=""
      accountStartDatePending={false}
      accountStartDateError={false}
      today={today}
      isAdminView
      onBackfillAbsence={props.onBackfillAbsence}
    />,
  );
}

function openMondayMenu() {
  const row = screen.getByText("05.01.").closest("tr");
  expect(row).not.toBeNull();
  fireEvent.click(within(row!).getByRole("button", { name: /^Aktionen für/ }));
}

describe("StaffSessionTable Abwesenheit nachtragen", () => {
  it("bietet auf einem Tag ohne Eintrag die Abwesenheit an", () => {
    const onBackfillAbsence = vi.fn();
    renderTable({ onBackfillAbsence });

    openMondayMenu();
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Abwesenheit nachtragen" }),
    );

    expect(onBackfillAbsence).toHaveBeenCalledTimes(1);
    const day = onBackfillAbsence.mock.calls[0]![0] as Date;
    expect([day.getFullYear(), day.getMonth(), day.getDate()]).toEqual([
      2026, 0, 5,
    ]);
  });

  it("bietet sie nicht an, wenn der Tag schon Arbeitszeit hat", () => {
    renderTable({ sessions: [mondaySession], onBackfillAbsence: vi.fn() });

    openMondayMenu();
    expect(
      screen.queryByRole("menuitem", { name: "Abwesenheit nachtragen" }),
    ).not.toBeInTheDocument();
  });

  it("bietet sie nicht an, wenn der Tag schon eine Abwesenheit hat", () => {
    renderTable({ absences: [mondayHalfSick], onBackfillAbsence: vi.fn() });

    openMondayMenu();
    expect(
      screen.queryByRole("menuitem", { name: "Abwesenheit nachtragen" }),
    ).not.toBeInTheDocument();
  });

  it("bietet sie ohne Berechtigung zum Eintragen nicht an", () => {
    renderTable({});

    openMondayMenu();
    expect(
      screen.queryByRole("menuitem", { name: "Abwesenheit nachtragen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "Eintrag nachtragen" }),
    ).toBeInTheDocument();
  });
});
