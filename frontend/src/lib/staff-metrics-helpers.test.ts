import { describe, expect, it } from "vitest";

import type { StaffAbsence, WorkSessionHistory } from "./time-tracking-helpers";
import type { DayProjection } from "./time-tracking-helpers";
import {
  adaptAbsenceForMetrics,
  adaptHistorySessionForMetrics,
  getDeltaStatus,
  periodTotalsFromProjection,
  resolveWeekIndex,
} from "./staff-metrics-helpers";

function day(overrides: Partial<DayProjection>): DayProjection {
  return {
    targetMinutes: 0,
    creditMinutes: 0,
    actualMinutes: 0,
    balanceMinutes: 0,
    ...overrides,
  };
}

describe("periodTotalsFromProjection", () => {
  // Mon worked, Tue on vacation, Wed (today) half worked, Thu/Fri still ahead.
  // The server prices every past day and leaves future days without Ist and
  // Saldo; the week reads that answer instead of crediting absences itself
  // (#6847), so it cannot disagree with the Monatskarte.
  it("sums the server's days: full Soll, Ist with credit, Saldo to today", () => {
    const projection = new Map<string, DayProjection>([
      [
        "2026-08-03",
        day({ targetMinutes: 480, actualMinutes: 500, balanceMinutes: 20 }),
      ],
      ["2026-08-04", day({ targetMinutes: 480, creditMinutes: 480 })],
      [
        "2026-08-05",
        day({ targetMinutes: 480, actualMinutes: 240, balanceMinutes: -240 }),
      ],
      ["2026-08-06", day({ targetMinutes: 480 })],
      ["2026-08-07", day({ targetMinutes: 480 })],
    ]);

    expect(periodTotalsFromProjection(projection)).toEqual({
      soll: 2400,
      ist: 500 + 480 + 240,
      delta: 20 + 0 - 240,
    });
  });

  // Freizeitausgleich credits nothing and charges the day's Soll; the server
  // says so in the day's balance, and the week must not soften it.
  it("keeps a server-charged comp-time day as charged", () => {
    const projection = new Map([
      ["2026-08-03", day({ targetMinutes: 480, balanceMinutes: -480 })],
    ]);

    expect(periodTotalsFromProjection(projection)).toEqual({
      soll: 480,
      ist: 0,
      delta: -480,
    });
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

  // #6851, #6852: the own Zeiterfassung shows the same credit hints as the
  // admin view, so the adapter must not drop them.
  it("keeps the pack-up time and rounding of the block", () => {
    const adapted = adaptHistorySessionForMetrics(
      historySession({ packUpMinutes: 5, roundingMinutes: -3 }),
    );
    expect(adapted.pack_up_minutes).toBe(5);
    expect(adapted.rounding_minutes).toBe(-3);
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
