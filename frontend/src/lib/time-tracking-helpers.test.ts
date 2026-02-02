import { describe, it, expect } from "vitest";
import type {
  BackendWorkSession,
  BackendWorkSessionBreak,
  BackendWorkSessionHistory,
  BackendWorkSessionEdit,
  BackendStaffAbsence,
  WorkSessionHistory,
} from "./time-tracking-helpers";
import {
  mapWorkSessionResponse,
  mapWorkSessionBreakResponse,
  mapWorkSessionHistoryResponse,
  mapWorkSessionEditResponse,
  mapStaffAbsenceResponse,
  formatDuration,
  formatTime,
  getWeekDays,
  getWeekNumber,
  getComplianceWarnings,
  calculateNetMinutes,
} from "./time-tracking-helpers";

// Helper to create mock backend session
const createMockBackendSession = (
  overrides: Partial<BackendWorkSession> = {},
): BackendWorkSession => ({
  id: 42,
  staff_id: 100,
  date: "2026-01-15T00:00:00Z",
  status: "present",
  check_in_time: "2026-01-15T08:00:00Z",
  check_out_time: "2026-01-15T16:00:00Z",
  break_minutes: 30,
  notes: "Test notes",
  auto_checked_out: false,
  created_by: 100,
  updated_by: null,
  created_at: "2026-01-15T08:00:00Z",
  updated_at: "2026-01-15T16:00:00Z",
  ...overrides,
});

describe("mapWorkSessionResponse", () => {
  it("converts IDs from number to string", () => {
    const backend = createMockBackendSession();
    const result = mapWorkSessionResponse(backend);

    expect(result.id).toBe("42");
    expect(result.staffId).toBe("100");
    expect(result.createdBy).toBe("100");
  });

  it("converts snake_case to camelCase", () => {
    const backend = createMockBackendSession();
    const result = mapWorkSessionResponse(backend);

    expect(result.checkInTime).toBe("2026-01-15T08:00:00Z");
    expect(result.checkOutTime).toBe("2026-01-15T16:00:00Z");
    expect(result.breakMinutes).toBe(30);
    expect(result.autoCheckedOut).toBe(false);
    expect(result.createdAt).toBe("2026-01-15T08:00:00Z");
    expect(result.updatedAt).toBe("2026-01-15T16:00:00Z");
  });

  it("splits date field on T to get date portion only", () => {
    const backend = createMockBackendSession({
      date: "2026-01-15T12:34:56Z",
    });
    const result = mapWorkSessionResponse(backend);

    expect(result.date).toBe("2026-01-15");
  });

  it("handles null check_out_time", () => {
    const backend = createMockBackendSession({ check_out_time: null });
    const result = mapWorkSessionResponse(backend);

    expect(result.checkOutTime).toBeNull();
  });

  it("handles null updated_by", () => {
    const backend = createMockBackendSession({ updated_by: null });
    const result = mapWorkSessionResponse(backend);

    expect(result.updatedBy).toBeNull();
  });

  it("handles empty notes string", () => {
    const backend = createMockBackendSession({ notes: "" });
    const result = mapWorkSessionResponse(backend);

    expect(result.notes).toBe("");
  });

  it("preserves home_office status", () => {
    const backend = createMockBackendSession({ status: "home_office" });
    const result = mapWorkSessionResponse(backend);

    expect(result.status).toBe("home_office");
  });
});

describe("mapWorkSessionBreakResponse", () => {
  it("converts IDs from number to string", () => {
    const backend: BackendWorkSessionBreak = {
      id: 5,
      session_id: 42,
      started_at: "2026-01-15T12:00:00Z",
      ended_at: "2026-01-15T12:30:00Z",
      duration_minutes: 30,
      created_at: "2026-01-15T12:00:00Z",
      updated_at: "2026-01-15T12:30:00Z",
    };
    const result = mapWorkSessionBreakResponse(backend);

    expect(result.id).toBe("5");
    expect(result.sessionId).toBe("42");
  });

  it("converts snake_case to camelCase", () => {
    const backend: BackendWorkSessionBreak = {
      id: 5,
      session_id: 42,
      started_at: "2026-01-15T12:00:00Z",
      ended_at: "2026-01-15T12:30:00Z",
      duration_minutes: 30,
      created_at: "2026-01-15T12:00:00Z",
      updated_at: "2026-01-15T12:30:00Z",
    };
    const result = mapWorkSessionBreakResponse(backend);

    expect(result.startedAt).toBe("2026-01-15T12:00:00Z");
    expect(result.endedAt).toBe("2026-01-15T12:30:00Z");
    expect(result.durationMinutes).toBe(30);
  });

  it("handles null ended_at", () => {
    const backend: BackendWorkSessionBreak = {
      id: 5,
      session_id: 42,
      started_at: "2026-01-15T12:00:00Z",
      ended_at: null,
      duration_minutes: 0,
      created_at: "2026-01-15T12:00:00Z",
      updated_at: "2026-01-15T12:00:00Z",
    };
    const result = mapWorkSessionBreakResponse(backend);

    expect(result.endedAt).toBeNull();
  });
});

describe("mapWorkSessionHistoryResponse", () => {
  it("maps base session and adds history fields", () => {
    const backend: BackendWorkSessionHistory = {
      ...createMockBackendSession(),
      net_minutes: 450,
      is_overtime: false,
      is_break_compliant: true,
      breaks: null,
      edit_count: 0,
    };
    const result = mapWorkSessionHistoryResponse(backend);

    expect(result.id).toBe("42");
    expect(result.netMinutes).toBe(450);
    expect(result.isOvertime).toBe(false);
    expect(result.isBreakCompliant).toBe(true);
    expect(result.editCount).toBe(0);
  });

  it("maps nested breaks array", () => {
    const backend: BackendWorkSessionHistory = {
      ...createMockBackendSession(),
      net_minutes: 450,
      is_overtime: false,
      is_break_compliant: true,
      breaks: [
        {
          id: 1,
          session_id: 42,
          started_at: "2026-01-15T12:00:00Z",
          ended_at: "2026-01-15T12:15:00Z",
          duration_minutes: 15,
          created_at: "2026-01-15T12:00:00Z",
          updated_at: "2026-01-15T12:15:00Z",
        },
        {
          id: 2,
          session_id: 42,
          started_at: "2026-01-15T14:00:00Z",
          ended_at: "2026-01-15T14:15:00Z",
          duration_minutes: 15,
          created_at: "2026-01-15T14:00:00Z",
          updated_at: "2026-01-15T14:15:00Z",
        },
      ],
      edit_count: 0,
    };
    const result = mapWorkSessionHistoryResponse(backend);

    expect(result.breaks).toHaveLength(2);
    expect(result.breaks[0]?.id).toBe("1");
    expect(result.breaks[0]?.sessionId).toBe("42");
    expect(result.breaks[1]?.id).toBe("2");
  });

  it("handles null breaks array", () => {
    const backend: BackendWorkSessionHistory = {
      ...createMockBackendSession(),
      net_minutes: 450,
      is_overtime: false,
      is_break_compliant: true,
      breaks: null,
      edit_count: 0,
    };
    const result = mapWorkSessionHistoryResponse(backend);

    expect(result.breaks).toEqual([]);
  });

  it("defaults edit_count to 0 when undefined", () => {
    const backend: BackendWorkSessionHistory = {
      ...createMockBackendSession(),
      net_minutes: 450,
      is_overtime: false,
      is_break_compliant: true,
      breaks: null,
      edit_count: undefined as unknown as number,
    };
    const result = mapWorkSessionHistoryResponse(backend);

    expect(result.editCount).toBe(0);
  });
});

describe("mapWorkSessionEditResponse", () => {
  it("converts IDs from number to string", () => {
    const backend: BackendWorkSessionEdit = {
      id: 10,
      session_id: 42,
      staff_id: 100,
      edited_by: 200,
      field_name: "check_out_time",
      old_value: "2026-01-15T16:00:00Z",
      new_value: "2026-01-15T17:00:00Z",
      notes: "Corrected checkout",
      created_at: "2026-01-15T17:05:00Z",
    };
    const result = mapWorkSessionEditResponse(backend);

    expect(result.id).toBe("10");
    expect(result.sessionId).toBe("42");
    expect(result.staffId).toBe("100");
    expect(result.editedBy).toBe("200");
  });

  it("converts snake_case to camelCase", () => {
    const backend: BackendWorkSessionEdit = {
      id: 10,
      session_id: 42,
      staff_id: 100,
      edited_by: 200,
      field_name: "check_out_time",
      old_value: "2026-01-15T16:00:00Z",
      new_value: "2026-01-15T17:00:00Z",
      notes: "Corrected checkout",
      created_at: "2026-01-15T17:05:00Z",
    };
    const result = mapWorkSessionEditResponse(backend);

    expect(result.fieldName).toBe("check_out_time");
    expect(result.oldValue).toBe("2026-01-15T16:00:00Z");
    expect(result.newValue).toBe("2026-01-15T17:00:00Z");
    expect(result.createdAt).toBe("2026-01-15T17:05:00Z");
  });

  it("handles null old_value and new_value", () => {
    const backend: BackendWorkSessionEdit = {
      id: 10,
      session_id: 42,
      staff_id: 100,
      edited_by: 200,
      field_name: "notes",
      old_value: null,
      new_value: null,
      notes: null,
      created_at: "2026-01-15T17:05:00Z",
    };
    const result = mapWorkSessionEditResponse(backend);

    expect(result.oldValue).toBeNull();
    expect(result.newValue).toBeNull();
    expect(result.notes).toBeNull();
  });
});

describe("mapStaffAbsenceResponse", () => {
  it("converts IDs from number to string", () => {
    const backend: BackendStaffAbsence = {
      id: 15,
      staff_id: 100,
      absence_type: "vacation",
      date_start: "2026-02-01T00:00:00Z",
      date_end: "2026-02-05T00:00:00Z",
      half_day: false,
      note: "Beach vacation",
      status: "approved",
      approved_by: 200,
      approved_at: "2026-01-20T10:00:00Z",
      created_by: 100,
      created_at: "2026-01-15T09:00:00Z",
      updated_at: "2026-01-20T10:00:00Z",
      duration_days: 5,
    };
    const result = mapStaffAbsenceResponse(backend);

    expect(result.id).toBe("15");
    expect(result.staffId).toBe("100");
    expect(result.approvedBy).toBe("200");
    expect(result.createdBy).toBe("100");
  });

  it("converts snake_case to camelCase", () => {
    const backend: BackendStaffAbsence = {
      id: 15,
      staff_id: 100,
      absence_type: "sick",
      date_start: "2026-02-01T00:00:00Z",
      date_end: "2026-02-05T00:00:00Z",
      half_day: true,
      note: "Flu",
      status: "pending",
      approved_by: null,
      approved_at: null,
      created_by: 100,
      created_at: "2026-01-15T09:00:00Z",
      updated_at: "2026-01-15T09:00:00Z",
      duration_days: 5,
    };
    const result = mapStaffAbsenceResponse(backend);

    expect(result.absenceType).toBe("sick");
    expect(result.halfDay).toBe(true);
    expect(result.approvedBy).toBeNull();
    expect(result.approvedAt).toBeNull();
    expect(result.durationDays).toBe(5);
  });

  it("splits date fields on T to get date portion only", () => {
    const backend: BackendStaffAbsence = {
      id: 15,
      staff_id: 100,
      absence_type: "training",
      date_start: "2026-02-01T08:00:00Z",
      date_end: "2026-02-05T17:00:00Z",
      half_day: false,
      note: "Conference",
      status: "approved",
      approved_by: 200,
      approved_at: "2026-01-20T10:00:00Z",
      created_by: 100,
      created_at: "2026-01-15T09:00:00Z",
      updated_at: "2026-01-20T10:00:00Z",
      duration_days: 5,
    };
    const result = mapStaffAbsenceResponse(backend);

    expect(result.dateStart).toBe("2026-02-01");
    expect(result.dateEnd).toBe("2026-02-05");
  });

  it("handles empty note string as empty string", () => {
    const backend: BackendStaffAbsence = {
      id: 15,
      staff_id: 100,
      absence_type: "other",
      date_start: "2026-02-01T00:00:00Z",
      date_end: "2026-02-01T00:00:00Z",
      half_day: false,
      note: "",
      status: "pending",
      approved_by: null,
      approved_at: null,
      created_by: 100,
      created_at: "2026-01-15T09:00:00Z",
      updated_at: "2026-01-15T09:00:00Z",
      duration_days: 1,
    };
    const result = mapStaffAbsenceResponse(backend);

    expect(result.note).toBe("");
  });
});

describe("formatDuration", () => {
  it("returns -- for null", () => {
    expect(formatDuration(null)).toBe("--");
  });

  it("returns -- for undefined", () => {
    expect(formatDuration(undefined)).toBe("--");
  });

  it("returns -- for NaN", () => {
    expect(formatDuration(Number.NaN)).toBe("--");
  });

  it("returns 0min for 0", () => {
    expect(formatDuration(0)).toBe("0min");
  });

  it("formats minutes only for values under 60", () => {
    expect(formatDuration(30)).toBe("30min");
    expect(formatDuration(45)).toBe("45min");
    expect(formatDuration(59)).toBe("59min");
  });

  it("formats hours only for exact hour values", () => {
    expect(formatDuration(60)).toBe("1h");
    expect(formatDuration(120)).toBe("2h");
    expect(formatDuration(480)).toBe("8h");
  });

  it("formats hours and minutes for mixed values", () => {
    expect(formatDuration(90)).toBe("1h 30min");
    expect(formatDuration(135)).toBe("2h 15min");
    expect(formatDuration(450)).toBe("7h 30min");
  });
});

describe("formatTime", () => {
  it("returns --:-- for null", () => {
    expect(formatTime(null)).toBe("--:--");
  });

  it("returns --:-- for undefined", () => {
    expect(formatTime(undefined)).toBe("--:--");
  });

  it("returns --:-- for invalid ISO string", () => {
    expect(formatTime("not-a-date")).toBe("--:--");
    expect(formatTime("2026-13-45T99:99:99Z")).toBe("--:--");
  });

  it("formats valid ISO timestamp to HH:MM in local time", () => {
    // formatTime converts UTC to local time using getHours()/getMinutes()
    const result1 = formatTime("2026-01-15T08:00:00Z");
    const result2 = formatTime("2026-01-15T14:30:00Z");
    const result3 = formatTime("2026-01-15T23:59:00Z");

    // Verify format is HH:MM
    expect(result1).toMatch(/^\d{2}:\d{2}$/);
    expect(result2).toMatch(/^\d{2}:\d{2}$/);
    expect(result3).toMatch(/^\d{2}:\d{2}$/);

    // Verify actual conversion matches Date behavior
    expect(result1).toBe(
      new Date("2026-01-15T08:00:00Z").getHours().toString().padStart(2, "0") +
        ":" +
        new Date("2026-01-15T08:00:00Z")
          .getMinutes()
          .toString()
          .padStart(2, "0"),
    );
  });

  it("pads single-digit hours and minutes with zero", () => {
    const result1 = formatTime("2026-01-15T08:05:00Z");
    const result2 = formatTime("2026-01-15T00:00:00Z");

    // Verify format is always HH:MM with zero padding
    expect(result1).toMatch(/^\d{2}:\d{2}$/);
    expect(result2).toMatch(/^\d{2}:\d{2}$/);
  });
});

describe("getWeekDays", () => {
  it("returns 7 days starting from Monday", () => {
    const date = new Date("2026-01-15"); // Thursday
    const days = getWeekDays(date);

    expect(days).toHaveLength(7);
    expect(days[0]?.getDay()).toBe(1); // Monday
    expect(days[6]?.getDay()).toBe(0); // Sunday
  });

  it("handles Monday input correctly", () => {
    // Use Date.UTC to avoid timezone issues
    const monday = new Date(Date.UTC(2026, 0, 19)); // Monday 2026-01-19 in UTC
    const days = getWeekDays(monday);

    // First day should be a Monday
    expect(days[0]?.getDay()).toBe(1); // Monday
    // All 7 days should be in sequence
    for (let i = 1; i < 7; i++) {
      const expectedDay = (i + 1) % 7; // Mon=1, Tue=2, ..., Sun=0
      expect(days[i]?.getDay()).toBe(expectedDay);
    }
  });

  it("handles Sunday input correctly", () => {
    // Use Date.UTC to avoid timezone issues
    const sunday = new Date(Date.UTC(2026, 0, 25)); // Sunday 2026-01-25 in UTC
    const days = getWeekDays(sunday);

    // Should return Monday of the same week
    expect(days[0]?.getDay()).toBe(1); // Monday
    expect(days[6]?.getDay()).toBe(0); // Sunday
    // Verify the Sunday is the same date we input
    const sundayResult = days[6];
    if (sundayResult) {
      expect(sundayResult.getFullYear()).toBe(2026);
      expect(sundayResult.getMonth()).toBe(0); // January
      expect(sundayResult.getDate()).toBe(25);
    }
  });

  it("normalizes times to midnight", () => {
    const date = new Date("2026-01-15T14:30:00Z");
    const days = getWeekDays(date);

    days.forEach((day) => {
      expect(day.getHours()).toBe(0);
      expect(day.getMinutes()).toBe(0);
      expect(day.getSeconds()).toBe(0);
      expect(day.getMilliseconds()).toBe(0);
    });
  });
});

describe("getWeekNumber", () => {
  it("returns ISO week number 1 for first week of year", () => {
    const date = new Date("2026-01-01"); // Thursday
    expect(getWeekNumber(date)).toBe(1);
  });

  it("returns correct week number for mid-year", () => {
    const date = new Date("2026-06-15"); // Monday
    expect(getWeekNumber(date)).toBeGreaterThan(20);
  });

  it("handles year boundary correctly", () => {
    const date = new Date("2025-12-29"); // Monday (ISO week 1 of 2026)
    const weekNum = getWeekNumber(date);
    expect(weekNum).toBeGreaterThan(0);
    expect(weekNum).toBeLessThanOrEqual(53);
  });
});

describe("getComplianceWarnings", () => {
  const createHistorySession = (
    overrides: Partial<WorkSessionHistory>,
  ): WorkSessionHistory => ({
    id: "1",
    staffId: "100",
    date: "2026-01-15",
    status: "present",
    checkInTime: "2026-01-15T08:00:00Z",
    checkOutTime: "2026-01-15T16:00:00Z",
    breakMinutes: 30,
    notes: "",
    autoCheckedOut: false,
    createdBy: "100",
    updatedBy: null,
    createdAt: "2026-01-15T08:00:00Z",
    updatedAt: "2026-01-15T16:00:00Z",
    netMinutes: 450,
    isOvertime: false,
    isBreakCompliant: true,
    breaks: [],
    editCount: 0,
    ...overrides,
  });

  it("returns warning for >9h work with <45min break", () => {
    const session = createHistorySession({
      netMinutes: 550, // >540 (9h)
      breakMinutes: 30, // <45
      isBreakCompliant: false,
    });
    const warnings = getComplianceWarnings(session);

    expect(warnings).toContain("Pausenzeit < 45min bei >9h Arbeitszeit");
  });

  it("returns warning for >6h work with <30min break", () => {
    const session = createHistorySession({
      netMinutes: 400, // >360 (6h)
      breakMinutes: 15, // <30
      isBreakCompliant: false,
    });
    const warnings = getComplianceWarnings(session);

    expect(warnings).toContain("Pausenzeit < 30min bei >6h Arbeitszeit");
  });

  it("returns auto-checkout warning", () => {
    const session = createHistorySession({
      autoCheckedOut: true,
    });
    const warnings = getComplianceWarnings(session);

    expect(warnings).toContain("Automatisch ausgestempelt");
  });

  it("returns multiple warnings when applicable", () => {
    const session = createHistorySession({
      netMinutes: 550,
      breakMinutes: 30,
      isBreakCompliant: false,
      autoCheckedOut: true,
    });
    const warnings = getComplianceWarnings(session);

    expect(warnings).toHaveLength(2);
    expect(warnings).toContain("Pausenzeit < 45min bei >9h Arbeitszeit");
    expect(warnings).toContain("Automatisch ausgestempelt");
  });

  it("returns empty array for compliant session", () => {
    const session = createHistorySession({
      netMinutes: 450,
      breakMinutes: 45,
      isBreakCompliant: true,
      autoCheckedOut: false,
    });
    const warnings = getComplianceWarnings(session);

    expect(warnings).toEqual([]);
  });

  it("does not warn when netMinutes is 0 or negative", () => {
    const session = createHistorySession({
      netMinutes: 0,
      breakMinutes: 0,
      isBreakCompliant: false,
    });
    const warnings = getComplianceWarnings(session);

    expect(warnings).toEqual([]);
  });
});

describe("calculateNetMinutes", () => {
  it("calculates net minutes for valid check-in/out with break", () => {
    const checkIn = "2026-01-15T08:00:00Z";
    const checkOut = "2026-01-15T16:00:00Z";
    const breakMinutes = 30;

    const result = calculateNetMinutes(checkIn, checkOut, breakMinutes);

    // 8 hours = 480 minutes, minus 30 break = 450
    expect(result).toBe(450);
  });

  it("returns null when checkout is null", () => {
    const checkIn = "2026-01-15T08:00:00Z";
    const result = calculateNetMinutes(checkIn, null, 0);

    expect(result).toBeNull();
  });

  it("returns null for invalid check-in date", () => {
    const checkIn = "invalid-date";
    const checkOut = "2026-01-15T16:00:00Z";

    const result = calculateNetMinutes(checkIn, checkOut, 0);

    expect(result).toBeNull();
  });

  it("returns null for invalid check-out date", () => {
    const checkIn = "2026-01-15T08:00:00Z";
    const checkOut = "invalid-date";

    const result = calculateNetMinutes(checkIn, checkOut, 0);

    expect(result).toBeNull();
  });

  it("returns 0 when break exceeds total time", () => {
    const checkIn = "2026-01-15T08:00:00Z";
    const checkOut = "2026-01-15T09:00:00Z"; // 1 hour
    const breakMinutes = 120; // 2 hours

    const result = calculateNetMinutes(checkIn, checkOut, breakMinutes);

    expect(result).toBe(0); // Math.max(0, ...)
  });

  it("handles same check-in and check-out time", () => {
    const time = "2026-01-15T08:00:00Z";
    const result = calculateNetMinutes(time, time, 0);

    expect(result).toBe(0);
  });

  it("calculates correctly with no break", () => {
    const checkIn = "2026-01-15T08:00:00Z";
    const checkOut = "2026-01-15T12:00:00Z"; // 4 hours

    const result = calculateNetMinutes(checkIn, checkOut, 0);

    expect(result).toBe(240); // 4 * 60
  });
});
