import { describe, expect, it } from "vitest";

import type {
  StaffAbsenceRow,
  StaffHistorySession,
  StaffSchedule,
} from "./staff-api";
import {
  computePeriodTotalsFromTargets,
  computeStaffMetrics,
  getDeltaStatus,
  resolveWeekIndex,
  toDateKey,
} from "./staff-metrics-helpers";

const schedule: StaffSchedule = {
  mode: "custom",
  model: null,
  rotationLength: 1,
  rotationAnchorDate: "2026-06-01",
  validFrom: "2026-06-01",
  weeklyTotals: [2400],
  entries: [0, 1, 2, 3, 4].map((dayOfWeek) => ({
    weekIndex: 0,
    dayOfWeek,
    targetMinutes: 480,
  })),
};

function absence(overrides: Partial<StaffAbsenceRow>): StaffAbsenceRow {
  return {
    id: 100,
    staff_id: 100,
    absence_type: "vacation",
    date_start: "2026-06-01",
    date_end: "2026-06-05",
    half_day: false,
    note: "",
    status: "approved",
    ...overrides,
  };
}

function session(overrides: Partial<StaffHistorySession>): StaffHistorySession {
  return {
    date: "2026-08-03",
    status: "present",
    net_minutes: 480,
    check_in_time: "08:00",
    check_out_time: "16:00",
    break_minutes: 0,
    ...overrides,
  };
}

describe("computeStaffMetrics absence credit", () => {
  it("credits a single half day absence with half the target", () => {
    const metrics = computeStaffMetrics(
      schedule,
      [],
      [
        absence({
          date_start: "2026-06-02",
          date_end: "2026-06-02",
          half_day: true,
        }),
      ],
      new Date(2026, 5, 5),
    );

    expect(metrics.weekIst).toBe(240);
    expect(metrics.weekDelta).toBe(-2160);
  });

  it("uses boundary half days instead of halving the whole range", () => {
    const metrics = computeStaffMetrics(
      schedule,
      [],
      [
        absence({
          half_day: true,
          start_half_day: true,
          end_half_day: false,
        }),
      ],
      new Date(2026, 5, 5),
    );

    expect(metrics.weekIst).toBe(2160);
    expect(metrics.weekDelta).toBe(-240);
  });

  it("does not credit pending, declined, or canceled absences", () => {
    const metrics = computeStaffMetrics(
      schedule,
      [],
      [
        absence({ status: "requested" }),
        absence({ status: "declined" }),
        absence({ status: "canceled" }),
      ],
      new Date(2026, 5, 5),
    );

    expect(metrics.weekIst).toBe(0);
    expect(metrics.weekDelta).toBe(-2400);
  });

  it("credits overlapping absences only once per day", () => {
    const metrics = computeStaffMetrics(
      schedule,
      [],
      [
        absence({ date_start: "2026-06-01", date_end: "2026-06-03" }),
        absence({
          absence_type: "sick",
          date_start: "2026-06-02",
          date_end: "2026-06-04",
        }),
      ],
      new Date(2026, 5, 5),
    );

    expect(metrics.weekIst).toBe(1920);
    expect(metrics.weekDelta).toBe(-480);
  });

  it("does not credit absences before the schedule validFrom date", () => {
    const metrics = computeStaffMetrics(
      { ...schedule, validFrom: "2026-06-03" },
      [],
      [absence({ date_start: "2026-06-01", date_end: "2026-06-05" })],
      new Date(2026, 5, 5),
    );

    expect(metrics.weekSoll).toBe(1440);
    expect(metrics.weekIst).toBe(1440);
    expect(metrics.weekDelta).toBe(0);
  });
});

describe("computeStaffMetrics account start", () => {
  it("uses Jan 1 when no account start date is configured", () => {
    const metrics = computeStaffMetrics(
      { ...schedule, validFrom: "2026-06-01" },
      [],
      [],
      new Date(2026, 0, 5),
    );

    expect(toDateKey(metrics.accountStart)).toBe("2026-01-01");
    expect(metrics.accountSoll).toBe(1440);
  });

  it("uses the configured account start date", () => {
    const metrics = computeStaffMetrics(
      schedule,
      [
        session({ date: "2026-07-31", net_minutes: 480 }),
        session({ date: "2026-08-03", net_minutes: 480 }),
      ],
      [],
      new Date(2026, 7, 5),
      "2026-08-01",
    );

    expect(toDateKey(metrics.accountStart)).toBe("2026-08-01");
    expect(metrics.accountSoll).toBe(1440);
    expect(metrics.accountIst).toBe(480);
    expect(metrics.accountBalance).toBe(-960);
  });

  it("falls back to Jan 1 when the configured account start date is invalid", () => {
    const metrics = computeStaffMetrics(
      schedule,
      [],
      [],
      new Date(2026, 5, 5),
      "not-a-date",
    );

    expect(toDateKey(metrics.accountStart)).toBe("2026-01-01");
  });

  it("counts account absences before schedule validFrom when the account start allows it", () => {
    const metrics = computeStaffMetrics(
      { ...schedule, validFrom: "2026-06-03" },
      [],
      [absence({ date_start: "2026-06-01", date_end: "2026-06-02" })],
      new Date(2026, 5, 5),
      "2026-06-01",
    );

    expect(metrics.weekSoll).toBe(1440);
    expect(metrics.weekIst).toBe(0);
    expect(metrics.accountSoll).toBe(2400);
    expect(metrics.accountIst).toBe(960);
    expect(metrics.accountBalance).toBe(-1440);
  });
});

describe("computePeriodTotalsFromTargets", () => {
  // Mon 2026-08-03 .. Sun 2026-08-09. The contract drops from 8h to 4h on
  // Wednesday — exactly the case computeStaffMetrics gets wrong, because it
  // would price the whole week at whichever single schedule it was handed.
  const weekStart = new Date(2026, 7, 3);
  const weekEnd = new Date(2026, 7, 9);
  const targets = new Map([
    ["2026-08-03", 480],
    ["2026-08-04", 480],
    ["2026-08-05", 240],
    ["2026-08-06", 240],
    ["2026-08-07", 240],
    ["2026-08-08", 0],
    ["2026-08-09", 0],
  ]);

  it("prices each day at the Soll that was valid on that day", () => {
    const totals = computePeriodTotalsFromTargets(
      targets,
      [session({ date: "2026-08-03" }), session({ date: "2026-08-04" })],
      [],
      weekStart,
      weekEnd,
      weekEnd,
    );

    // 480+480+240+240+240 — not 5x480 and not 5x240.
    expect(totals.soll).toBe(1680);
    expect(totals.ist).toBe(960);
    expect(totals.delta).toBe(960 - 1680);
  });

  it("prices the delta against the Soll up to today, but Soll for the full week", () => {
    // Tuesday: Wed-Fri have not happened yet and must not read as Minusstunden.
    const tuesday = new Date(2026, 7, 4);
    const totals = computePeriodTotalsFromTargets(
      targets,
      [session({ date: "2026-08-03" }), session({ date: "2026-08-04" })],
      [],
      weekStart,
      weekEnd,
      tuesday,
    );

    expect(totals.soll).toBe(1680);
    expect(totals.delta).toBe(0);
  });

  it("credits an absence with the date-valid target of that day", () => {
    // Wednesday is a 4h day after the contract change, so Krank credits 240.
    const totals = computePeriodTotalsFromTargets(
      targets,
      [],
      [
        absence({
          absence_type: "sick",
          date_start: "2026-08-05",
          date_end: "2026-08-05",
        }),
      ],
      weekStart,
      weekEnd,
      weekEnd,
    );

    expect(totals.ist).toBe(240);
  });

  it("counts days outside the fetched range as zero Soll", () => {
    const totals = computePeriodTotalsFromTargets(
      new Map(),
      [],
      [],
      weekStart,
      weekEnd,
      weekEnd,
    );

    expect(totals.soll).toBe(0);
    expect(totals.delta).toBe(0);
  });
});

describe("resolveWeekIndex", () => {
  it("returns the forward rotation week index", () => {
    expect(
      resolveWeekIndex(
        { rotationLength: 2, rotationAnchorDate: "2026-06-01" },
        new Date(2026, 5, 8),
      ),
    ).toBe(1);
  });

  it("wraps dates before the rotation anchor into a positive index", () => {
    expect(
      resolveWeekIndex(
        { rotationLength: 3, rotationAnchorDate: "2026-06-15" },
        new Date(2026, 5, 1),
      ),
    ).toBe(1);
  });

  it("falls back to week zero when the rotation anchor is invalid", () => {
    expect(
      resolveWeekIndex(
        { rotationLength: 2, rotationAnchorDate: "invalid" },
        new Date(2026, 5, 8),
      ),
    ).toBe(0);
  });
});

describe("getDeltaStatus", () => {
  it("uses green inside tolerance, amber above, and gray below", () => {
    expect(getDeltaStatus(0)).toBe("green");
    expect(getDeltaStatus(15)).toBe("green");
    expect(getDeltaStatus(16)).toBe("amber");
    expect(getDeltaStatus(-16)).toBe("gray");
  });
});
