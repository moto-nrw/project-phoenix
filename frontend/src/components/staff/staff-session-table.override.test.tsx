import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { StaffSchedule } from "~/lib/staff-api";
import type { DayProjection } from "~/lib/time-tracking-helpers";

import { StaffSessionTable } from "./staff-session-table";

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

function projected(targetMinutes: number, isOverride = false): DayProjection {
  return {
    targetMinutes,
    creditMinutes: 0,
    actualMinutes: 0,
    balanceMinutes: -targetMinutes,
    ...(isOverride ? { isOverride: true } : {}),
  };
}

// Mo–Fr 05.–09.01.2026 are closing days. Monday carries a Sonderarbeitszeit of
// 8,5 h, Tuesday one of 0 h; Wednesday is a plain closing day.
function renderClosureWeek() {
  const closing = new Map(
    ["2026-01-05", "2026-01-06", "2026-01-07", "2026-01-08", "2026-01-09"].map(
      (date) => [date, "Herbstferien"] as const,
    ),
  );
  return render(
    <StaffSessionTable
      staffId="1"
      from={new Date(2026, 0, 5)}
      to={new Date(2026, 0, 9)}
      sessions={[]}
      schedule={schedule}
      dailyProjection={
        new Map([
          ["2026-01-05", projected(510, true)],
          ["2026-01-06", projected(0, true)],
          ["2026-01-07", projected(0)],
          ["2026-01-08", projected(0)],
          ["2026-01-09", projected(0)],
        ])
      }
      closingDays={closing}
      accountStartDate=""
      accountStartDatePending={false}
      accountStartDateError={false}
      today={new Date(2026, 5, 15)}
      isAdminView
    />,
  );
}

function row(date: string): HTMLElement {
  const cell = screen.getByText(date).closest("tr");
  if (!cell) throw new Error(`row ${date} missing`);
  return cell;
}

describe("StaffSessionTable Sonderarbeitszeit (#3259)", () => {
  it("shows the override Soll, names both sources and keeps „Nicht erfasst“", () => {
    renderClosureWeek();

    const monday = row("05.01.");
    expect(within(monday).getByText("8h 30min")).toBeInTheDocument();
    expect(
      within(monday).getByText("Sonderarbeitszeit · Schließtag"),
    ).toBeInTheDocument();
    expect(within(monday).getByText("Nicht erfasst")).toBeInTheDocument();
    expect(within(monday).queryByText("Schließtag")).toBeNull();
  });

  it("shows 0h on a 0-hour Sonderarbeitszeit, the closure badge stays", () => {
    renderClosureWeek();

    const tuesday = row("06.01.");
    expect(within(tuesday).getByText("0h")).toBeInTheDocument();
    expect(within(tuesday).getByText("Schließtag")).toBeInTheDocument();
    expect(within(tuesday).queryByText("Nicht erfasst")).toBeNull();
  });

  it("leaves a plain closing day without a Soll marker", () => {
    renderClosureWeek();

    const wednesday = row("07.01.");
    expect(within(wednesday).getByText("Schließtag")).toBeInTheDocument();
    expect(within(wednesday).queryByText(/Sonderarbeitszeit/)).toBeNull();
    expect(within(wednesday).queryByText("0h")).toBeNull();
  });
});
