import { afterEach, describe, expect, it, vi } from "vitest";

import type {
  StaffAbsenceRow,
  StaffHistorySession,
  StaffSchedule,
} from "./staff-api";
import type { StaffAbsence, WorkSessionHistory } from "./time-tracking-helpers";
import {
  adaptAbsenceForMetrics,
  adaptHistorySessionForMetrics,
  computePeriodTotalsFromTargets,
  computeStaffMetrics,
  getDeltaStatus,
  indexAbsenceCreditByDay,
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

afterEach(() => {
  vi.useRealTimers();
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

  it("counts only the Monday share of a block started on Sunday", () => {
    const monday = new Date(2026, 7, 3);
    const totals = computePeriodTotalsFromTargets(
      new Map([["2026-08-03", 480]]),
      [
        session({
          date: "2026-08-02",
          check_in_time: "2026-08-02T20:00:00.000Z", // 22:00 CEST
          check_out_time: "2026-08-03T00:00:00.000Z", // 02:00 CEST
          net_minutes: 240,
        }),
      ],
      [],
      monday,
      monday,
      monday,
    );

    expect(totals.ist).toBe(120);
    expect(totals.delta).toBe(-360);
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

  it("excludes future-dated sessions from Ist and delta", () => {
    // Admins may date a session later in the week. Its minutes must not read
    // as Überstunden against a Soll that deliberately stops at today.
    const tuesday = new Date(2026, 7, 4);
    const totals = computePeriodTotalsFromTargets(
      targets,
      [
        session({ date: "2026-08-03" }),
        session({ date: "2026-08-04" }),
        session({ date: "2026-08-06", net_minutes: 240 }),
      ],
      [],
      weekStart,
      weekEnd,
      tuesday,
    );

    expect(totals.ist).toBe(960);
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

  it("lets the lowest absence ID price an overlapping day, like the server", () => {
    // Wednesday is covered twice. The server credits it from the lowest
    // absence ID (`addAbsenceCredits`), so the half sick day wins and pays
    // 120 — half of the day's 240. The API returns absences by date, which
    // put the full vacation day first: crediting in arrival order paid the
    // full 240 and the week card drifted away from the Monatskarte (#1842).
    const totals = computePeriodTotalsFromTargets(
      targets,
      [],
      [
        absence({
          id: 2,
          date_start: "2026-08-05",
          date_end: "2026-08-05",
        }),
        absence({
          id: 1,
          absence_type: "sick",
          half_day: true,
          date_start: "2026-08-05",
          date_end: "2026-08-05",
        }),
      ],
      weekStart,
      weekEnd,
      weekEnd,
    );

    expect(totals.ist).toBe(120);
  });

  it("does not subtract a running break that net_minutes already deducts", () => {
    // /history computes `net_minutes` at request time and already deducts a
    // running break (netMinutesWithBreaks), the same math the Monatskarte
    // uses. Deducting it a second time here would count it twice and make the
    // week card understate Ist against the month card and the day row.
    vi.setSystemTime(new Date("2026-08-03T12:00:00Z"));
    const totals = computePeriodTotalsFromTargets(
      targets,
      [
        session({
          date: "2026-08-03",
          net_minutes: 90, // 10:00 -> now (120) minus 30 min of running break
          check_in_time: "2026-08-03T10:00:00Z",
          check_out_time: null,
          break_minutes: 0,
          breaks: [{ started_at: "2026-08-03T11:30:00Z", ended_at: null }],
        }),
      ],
      [],
      weekStart,
      weekEnd,
      new Date(2026, 7, 3),
    );

    expect(totals.ist).toBe(90);
  });

  it("does not subtract an ended break twice", () => {
    // An ended break is already deducted in `net_minutes`.
    vi.setSystemTime(new Date("2026-08-03T12:00:00Z"));
    const totals = computePeriodTotalsFromTargets(
      targets,
      [
        session({
          date: "2026-08-03",
          net_minutes: 450,
          break_minutes: 30,
          breaks: [
            {
              started_at: "2026-08-03T11:00:00Z",
              ended_at: "2026-08-03T11:30:00Z",
            },
          ],
        }),
      ],
      [],
      weekStart,
      weekEnd,
      new Date(2026, 7, 3),
    );

    expect(totals.ist).toBe(450);
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

describe("indexAbsenceCreditByDay", () => {
  // Mon–Fri, 480 min/day across the test week. A weekend / unknown day is 0.
  const targets = new Map<string, number>([
    ["2026-06-01", 480], // Mon
    ["2026-06-02", 480], // Tue
    ["2026-06-03", 480], // Wed
    ["2026-06-04", 480], // Thu
    ["2026-06-05", 480], // Fri
  ]);

  it("credits only reported/approved absences, ignoring requested/declined/canceled", () => {
    const byDay = indexAbsenceCreditByDay(targets, [
      absence({ date_start: "2026-06-01", date_end: "2026-06-01" }),
      absence({
        id: 200,
        date_start: "2026-06-02",
        date_end: "2026-06-02",
        status: "requested",
      }),
      absence({
        id: 201,
        date_start: "2026-06-03",
        date_end: "2026-06-03",
        status: "declined",
      }),
      absence({
        id: 202,
        date_start: "2026-06-04",
        date_end: "2026-06-04",
        status: "canceled",
      }),
    ]);

    expect(byDay.get("2026-06-01")).toBe(480);
    expect(byDay.has("2026-06-02")).toBe(false);
    expect(byDay.has("2026-06-03")).toBe(false);
    expect(byDay.has("2026-06-04")).toBe(false);
  });

  it("credits a half-day boundary with half the day's target", () => {
    const byDay = indexAbsenceCreditByDay(targets, [
      absence({
        date_start: "2026-06-01",
        date_end: "2026-06-01",
        half_day: true,
      }),
    ]);

    expect(byDay.get("2026-06-01")).toBe(240);
  });

  it("credits an overlapping day once with the lowest-ID absence", () => {
    const byDay = indexAbsenceCreditByDay(targets, [
      // Higher ID, half day — must NOT win.
      absence({
        id: 300,
        date_start: "2026-06-02",
        date_end: "2026-06-02",
        half_day: true,
      }),
      // Lower ID, full day — wins the overlap, mirroring the server.
      absence({
        id: 5,
        date_start: "2026-06-02",
        date_end: "2026-06-02",
      }),
    ]);

    expect(byDay.get("2026-06-02")).toBe(480);
  });

  it("credits 0 for a day without a target", () => {
    const byDay = indexAbsenceCreditByDay(targets, [
      absence({ date_start: "2026-06-06", date_end: "2026-06-06" }), // Sat, no target
    ]);

    expect(byDay.get("2026-06-06")).toBe(0);
  });
});

describe("adaptHistorySessionForMetrics", () => {
  const historySession = (
    overrides: Partial<WorkSessionHistory> = {},
  ): WorkSessionHistory => ({
    id: "42",
    staffId: "100",
    date: "2026-07-21",
    status: "home_office",
    checkInTime: "2026-07-21T13:30:00Z",
    checkOutTime: null,
    breakMinutes: 0,
    notes: "",
    autoCheckedOut: false,
    createdBy: "100",
    updatedBy: null,
    createdAt: "2026-07-21T13:30:00Z",
    updatedAt: "2026-07-21T13:30:00Z",
    netMinutes: 5,
    isOvertime: false,
    isBreakCompliant: true,
    restPeriodWarning: null,
    breaks: [],
    editCount: 0,
    ...overrides,
  });

  // A kiosk stamp must stay recognisable as one in the staff member's OWN
  // view, not just in the admin view: the source column previously dropped
  // the field here and rendered every NFC session as "App".
  it("keeps an NFC stamp attributed to NFC", () => {
    expect(
      adaptHistorySessionForMetrics(historySession({ source: "nfc" })).source,
    ).toBe("nfc");
  });

  it("keeps a legacy row's unknown source unknown", () => {
    expect(
      adaptHistorySessionForMetrics(historySession({ source: "unknown" }))
        .source,
    ).toBe("unknown");
  });
});

describe("adaptAbsenceForMetrics", () => {
  it("keeps the custom absence type identity and label", () => {
    const custom: StaffAbsence = {
      id: "100",
      staffId: "200",
      absenceType: "other",
      absenceTypeId: "9007199254740993",
      absenceTypeLabel: "Regenerationstag",
      dateStart: "2026-08-20",
      dateEnd: "2026-08-20",
      halfDay: false,
      startHalfDay: false,
      endHalfDay: false,
      note: "",
      status: "approved",
      approvedBy: null,
      approvedAt: null,
      createdBy: "200",
      createdAt: "2026-08-20T08:00:00Z",
      updatedAt: "2026-08-20T08:00:00Z",
      durationDays: 1,
      workingDays: 1,
      decisionNote: "",
      requestedAt: "2026-08-20T08:00:00Z",
      substituteStaffId: null,
    };

    expect(adaptAbsenceForMetrics(custom)).toMatchObject({
      absence_type_id: "9007199254740993",
      absence_type_label: "Regenerationstag",
    });
  });
});
