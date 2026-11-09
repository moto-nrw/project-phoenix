import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";

import type { StaffHistorySession, StaffSchedule } from "~/lib/staff-api";
import { setTestClock } from "~/test/clock";

import { StaffSessionTable } from "./staff-session-table";

// Runs in `school-date-dom`: only forked processes apply a runtime TZ change.
afterEach(() => vi.unstubAllEnvs());

// The table invalidates the cache of the signed-in session after a save;
// this test renders it without a SessionProvider.
vi.mock("~/lib/swr", () => ({ useInvalidateMatching: () => vi.fn() }));

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

// 20.10.2026 is summer time: 08:00–16:30 Berlin = 06:00–14:30 UTC.
const session: StaffHistorySession = {
  id: "42",
  date: "2026-10-20",
  net_minutes: 480,
  check_in_time: "2026-10-20T06:00:00Z",
  check_out_time: "2026-10-20T14:30:00Z",
  break_minutes: 30,
};

describe("StaffSessionTable in a browser outside Germany (#6293)", () => {
  it("shows check-in and check-out as Berlin time", () => {
    vi.stubEnv("TZ", "UTC");
    setTestClock("2026-11-09T12:00:00+01:00");
    // The zone change took effect, or the case below proves nothing.
    expect(new Date(session.check_in_time).getHours()).toBe(6);

    render(
      <StaffSessionTable
        staffId="7"
        from={new Date(2026, 9, 19)}
        to={new Date(2026, 9, 25)}
        sessions={[session]}
        schedule={schedule}
        accountStartDate=""
        accountStartDatePending={false}
        accountStartDateError={false}
        today={new Date(2026, 10, 9)}
        isAdminView
      />,
    );

    const row = screen.getByText("20.10.").closest("tr")!;
    expect(within(row).getByText("08:00")).toBeInTheDocument();
    expect(within(row).getByText("16:30")).toBeInTheDocument();
    expect(within(row).queryByText("06:00")).not.toBeInTheDocument();
  });
});
