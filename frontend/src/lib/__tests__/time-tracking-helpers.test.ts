import { describe, it, expect } from "vitest";
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
  type BackendWorkSession,
  type BackendWorkSessionBreak,
  type BackendWorkSessionHistory,
  type BackendWorkSessionEdit,
  type BackendStaffAbsence,
  type WorkSessionHistory,
} from "../time-tracking-helpers";

// ============================================================================
// mapWorkSessionResponse
// ============================================================================
describe("mapWorkSessionResponse", () => {
  const backendSession: BackendWorkSession = {
    id: 42,
    staff_id: 7,
    date: "2024-03-15T00:00:00Z",
    status: "present",
    check_in_time: "2024-03-15T08:00:00Z",
    check_out_time: "2024-03-15T16:00:00Z",
    break_minutes: 30,
    notes: "Normal day",
    auto_checked_out: false,
    created_by: 7,
    updated_by: null,
    created_at: "2024-03-15T08:00:00Z",
    updated_at: "2024-03-15T16:00:00Z",
  };

  it("maps all fields correctly", () => {
    const result = mapWorkSessionResponse(backendSession);
    expect(result.id).toBe("42");
    expect(result.staffId).toBe("7");
    expect(result.date).toBe("2024-03-15");
    expect(result.status).toBe("present");
    expect(result.checkInTime).toBe("2024-03-15T08:00:00Z");
    expect(result.checkOutTime).toBe("2024-03-15T16:00:00Z");
    expect(result.breakMinutes).toBe(30);
    expect(result.notes).toBe("Normal day");
    expect(result.autoCheckedOut).toBe(false);
    expect(result.createdBy).toBe("7");
    expect(result.updatedBy).toBeNull();
  });

  it("handles null check_out_time", () => {
    const active = { ...backendSession, check_out_time: null };
    const result = mapWorkSessionResponse(active);
    expect(result.checkOutTime).toBeNull();
  });

  it("handles updated_by", () => {
    const updated = { ...backendSession, updated_by: 9 };
    const result = mapWorkSessionResponse(updated);
    expect(result.updatedBy).toBe("9");
  });

  it("defaults notes to empty string", () => {
    const noNotes = { ...backendSession, notes: "" };
    const result = mapWorkSessionResponse(noNotes);
    expect(result.notes).toBe("");
  });

  it("strips time from date", () => {
    const session = { ...backendSession, date: "2024-03-15T12:30:00Z" };
    const result = mapWorkSessionResponse(session);
    expect(result.date).toBe("2024-03-15");
  });
});

// ============================================================================
// mapWorkSessionBreakResponse
// ============================================================================
describe("mapWorkSessionBreakResponse", () => {
  const backendBreak: BackendWorkSessionBreak = {
    id: 5,
    session_id: 42,
    started_at: "2024-03-15T12:00:00Z",
    ended_at: "2024-03-15T12:30:00Z",
    duration_minutes: 30,
    created_at: "2024-03-15T12:00:00Z",
    updated_at: "2024-03-15T12:30:00Z",
  };

  it("maps all fields correctly", () => {
    const result = mapWorkSessionBreakResponse(backendBreak);
    expect(result.id).toBe("5");
    expect(result.sessionId).toBe("42");
    expect(result.startedAt).toBe("2024-03-15T12:00:00Z");
    expect(result.endedAt).toBe("2024-03-15T12:30:00Z");
    expect(result.durationMinutes).toBe(30);
  });

  it("handles null ended_at (active break)", () => {
    const active = { ...backendBreak, ended_at: null };
    const result = mapWorkSessionBreakResponse(active);
    expect(result.endedAt).toBeNull();
  });
});

// ============================================================================
// mapWorkSessionHistoryResponse
// ============================================================================
describe("mapWorkSessionHistoryResponse", () => {
  const backendHistory: BackendWorkSessionHistory = {
    id: 42,
    staff_id: 7,
    date: "2024-03-15T00:00:00Z",
    status: "home_office",
    check_in_time: "2024-03-15T08:00:00Z",
    check_out_time: "2024-03-15T16:00:00Z",
    break_minutes: 30,
    notes: "",
    auto_checked_out: true,
    created_by: 7,
    updated_by: null,
    created_at: "2024-03-15T08:00:00Z",
    updated_at: "2024-03-15T16:00:00Z",
    net_minutes: 450,
    is_overtime: false,
    is_break_compliant: true,
    breaks: [
      {
        id: 1,
        session_id: 42,
        started_at: "2024-03-15T12:00:00Z",
        ended_at: "2024-03-15T12:30:00Z",
        duration_minutes: 30,
        created_at: "2024-03-15T12:00:00Z",
        updated_at: "2024-03-15T12:30:00Z",
      },
    ],
    edit_count: 2,
  };

  it("maps all fields including history-specific ones", () => {
    const result = mapWorkSessionHistoryResponse(backendHistory);
    expect(result.netMinutes).toBe(450);
    expect(result.isOvertime).toBe(false);
    expect(result.isBreakCompliant).toBe(true);
    expect(result.breaks).toHaveLength(1);
    expect(result.breaks[0]!.id).toBe("1");
    expect(result.editCount).toBe(2);
    expect(result.status).toBe("home_office");
    expect(result.autoCheckedOut).toBe(true);
  });

  it("handles null breaks array", () => {
    const noBreaks = { ...backendHistory, breaks: null };
    const result = mapWorkSessionHistoryResponse(noBreaks);
    expect(result.breaks).toEqual([]);
  });

  it("defaults edit_count to 0", () => {
    const noEdits = {
      ...backendHistory,
      edit_count: undefined,
    } as unknown as BackendWorkSessionHistory;
    const result = mapWorkSessionHistoryResponse(noEdits);
    expect(result.editCount).toBe(0);
  });
});

// ============================================================================
// mapWorkSessionEditResponse
// ============================================================================
describe("mapWorkSessionEditResponse", () => {
  const backendEdit: BackendWorkSessionEdit = {
    id: 10,
    session_id: 42,
    staff_id: 7,
    edited_by: 3,
    field_name: "check_in_time",
    old_value: "2024-03-15T08:00:00Z",
    new_value: "2024-03-15T07:30:00Z",
    notes: "Corrected check-in",
    created_at: "2024-03-15T17:00:00Z",
  };

  it("maps all fields correctly", () => {
    const result = mapWorkSessionEditResponse(backendEdit);
    expect(result.id).toBe("10");
    expect(result.sessionId).toBe("42");
    expect(result.staffId).toBe("7");
    expect(result.editedBy).toBe("3");
    expect(result.fieldName).toBe("check_in_time");
    expect(result.oldValue).toBe("2024-03-15T08:00:00Z");
    expect(result.newValue).toBe("2024-03-15T07:30:00Z");
    expect(result.notes).toBe("Corrected check-in");
  });

  it("handles null old_value and new_value", () => {
    const edit = { ...backendEdit, old_value: null, new_value: null };
    const result = mapWorkSessionEditResponse(edit);
    expect(result.oldValue).toBeNull();
    expect(result.newValue).toBeNull();
  });

  it("handles null notes", () => {
    const edit = { ...backendEdit, notes: null };
    const result = mapWorkSessionEditResponse(edit);
    expect(result.notes).toBeNull();
  });
});

// ============================================================================
// mapStaffAbsenceResponse
// ============================================================================
describe("mapStaffAbsenceResponse", () => {
  const backendAbsence: BackendStaffAbsence = {
    id: 20,
    staff_id: 7,
    absence_type: "sick",
    date_start: "2024-03-15T00:00:00Z",
    date_end: "2024-03-17T00:00:00Z",
    half_day: false,
    note: "Flu",
    status: "reported",
    approved_by: null,
    approved_at: null,
    created_by: 7,
    created_at: "2024-03-15T08:00:00Z",
    updated_at: "2024-03-15T08:00:00Z",
    duration_days: 3,
  };

  it("maps all fields correctly", () => {
    const result = mapStaffAbsenceResponse(backendAbsence);
    expect(result.id).toBe("20");
    expect(result.staffId).toBe("7");
    expect(result.absenceType).toBe("sick");
    expect(result.dateStart).toBe("2024-03-15");
    expect(result.dateEnd).toBe("2024-03-17");
    expect(result.halfDay).toBe(false);
    expect(result.note).toBe("Flu");
    expect(result.status).toBe("reported");
    expect(result.approvedBy).toBeNull();
    expect(result.approvedAt).toBeNull();
    expect(result.createdBy).toBe("7");
    expect(result.durationDays).toBe(3);
  });

  it("handles approved absence", () => {
    const approved = {
      ...backendAbsence,
      approved_by: 5,
      approved_at: "2024-03-16T10:00:00Z",
      status: "approved",
    };
    const result = mapStaffAbsenceResponse(approved);
    expect(result.approvedBy).toBe("5");
    expect(result.approvedAt).toBe("2024-03-16T10:00:00Z");
    expect(result.status).toBe("approved");
  });
});

// ============================================================================
// formatDuration
// ============================================================================
describe("formatDuration", () => {
  it("returns -- for null", () => {
    expect(formatDuration(null)).toBe("--");
  });

  it("returns -- for undefined", () => {
    expect(formatDuration(undefined)).toBe("--");
  });

  it("returns -- for NaN", () => {
    expect(formatDuration(NaN)).toBe("--");
  });

  it("returns 0min for 0", () => {
    expect(formatDuration(0)).toBe("0min");
  });

  it("formats minutes only", () => {
    expect(formatDuration(45)).toBe("45min");
  });

  it("formats hours only", () => {
    expect(formatDuration(120)).toBe("2h");
  });

  it("formats hours and minutes", () => {
    expect(formatDuration(390)).toBe("6h 30min");
  });

  it("formats single minute", () => {
    expect(formatDuration(1)).toBe("1min");
  });
});

// ============================================================================
// formatTime
// ============================================================================
describe("formatTime", () => {
  it("returns --:-- for null", () => {
    expect(formatTime(null)).toBe("--:--");
  });

  it("returns --:-- for undefined", () => {
    expect(formatTime(undefined)).toBe("--:--");
  });

  it("returns --:-- for empty string", () => {
    expect(formatTime("")).toBe("--:--");
  });

  it("returns --:-- for invalid date", () => {
    expect(formatTime("not-a-date")).toBe("--:--");
  });

  it("formats valid ISO string", () => {
    const result = formatTime("2024-03-15T08:05:00Z");
    // Result depends on timezone, but format should be HH:MM
    expect(result).toMatch(/^\d{2}:\d{2}$/);
  });
});

// ============================================================================
// getWeekDays
// ============================================================================
describe("getWeekDays", () => {
  it("returns 7 days", () => {
    const date = new Date(2024, 2, 15); // Friday March 15
    const days = getWeekDays(date);
    expect(days).toHaveLength(7);
  });

  it("starts on Monday", () => {
    const date = new Date(2024, 2, 15); // Friday March 15
    const days = getWeekDays(date);
    expect(days[0]!.getDay()).toBe(1); // Monday
  });

  it("ends on Sunday", () => {
    const date = new Date(2024, 2, 15);
    const days = getWeekDays(date);
    expect(days[6]!.getDay()).toBe(0); // Sunday
  });

  it("handles Sunday input", () => {
    const sunday = new Date(2024, 2, 17); // Sunday March 17
    const days = getWeekDays(sunday);
    expect(days[0]!.getDay()).toBe(1); // Monday
    expect(days).toHaveLength(7);
  });

  it("handles Monday input", () => {
    const monday = new Date(2024, 2, 11); // Monday March 11
    const days = getWeekDays(monday);
    expect(days[0]!.getDate()).toBe(11);
    expect(days[0]!.getDay()).toBe(1);
  });

  it("sets all times to midnight", () => {
    const date = new Date(2024, 2, 15, 14, 30);
    const days = getWeekDays(date);
    for (const d of days) {
      expect(d.getHours()).toBe(0);
      expect(d.getMinutes()).toBe(0);
    }
  });
});

// ============================================================================
// getWeekNumber
// ============================================================================
describe("getWeekNumber", () => {
  it("returns correct week number for known date", () => {
    // 2024-01-01 is Monday of ISO week 1
    const date = new Date(2024, 0, 1);
    expect(getWeekNumber(date)).toBe(1);
  });

  it("returns week 52 or 53 for late December", () => {
    const date = new Date(2024, 11, 28);
    const wn = getWeekNumber(date);
    expect(wn).toBeGreaterThanOrEqual(52);
  });

  it("returns correct week for mid-year date", () => {
    // 2024-03-15 is in week 11
    const date = new Date(2024, 2, 15);
    expect(getWeekNumber(date)).toBe(11);
  });
});

// ============================================================================
// getComplianceWarnings
// ============================================================================
describe("getComplianceWarnings", () => {
  const makeSession = (
    overrides: Partial<WorkSessionHistory>,
  ): WorkSessionHistory => ({
    id: "1",
    staffId: "1",
    date: "2024-03-15",
    status: "present",
    checkInTime: "2024-03-15T08:00:00Z",
    checkOutTime: "2024-03-15T16:00:00Z",
    breakMinutes: 30,
    notes: "",
    autoCheckedOut: false,
    createdBy: "1",
    updatedBy: null,
    createdAt: "2024-03-15T08:00:00Z",
    updatedAt: "2024-03-15T16:00:00Z",
    netMinutes: 450,
    isOvertime: false,
    isBreakCompliant: true,
    breaks: [],
    editCount: 0,
    ...overrides,
  });

  it("returns no warnings for compliant session", () => {
    const session = makeSession({});
    expect(getComplianceWarnings(session)).toEqual([]);
  });

  it("warns about insufficient break over 6h", () => {
    const session = makeSession({
      netMinutes: 400,
      breakMinutes: 15,
      isBreakCompliant: false,
    });
    const warnings = getComplianceWarnings(session);
    expect(warnings).toHaveLength(1);
    expect(warnings[0]).toContain("30min");
  });

  it("warns about insufficient break over 9h", () => {
    const session = makeSession({
      netMinutes: 560,
      breakMinutes: 30,
      isBreakCompliant: false,
    });
    const warnings = getComplianceWarnings(session);
    expect(warnings).toHaveLength(1);
    expect(warnings[0]).toContain("45min");
  });

  it("warns about auto checkout", () => {
    const session = makeSession({
      autoCheckedOut: true,
    });
    const warnings = getComplianceWarnings(session);
    expect(warnings).toHaveLength(1);
    expect(warnings[0]).toContain("Automatisch ausgestempelt");
  });

  it("returns multiple warnings", () => {
    const session = makeSession({
      netMinutes: 560,
      breakMinutes: 30,
      isBreakCompliant: false,
      autoCheckedOut: true,
    });
    const warnings = getComplianceWarnings(session);
    expect(warnings).toHaveLength(2);
  });

  it("no break warning when netMinutes is 0", () => {
    const session = makeSession({
      netMinutes: 0,
      breakMinutes: 0,
      isBreakCompliant: false,
    });
    const warnings = getComplianceWarnings(session);
    expect(warnings).toEqual([]);
  });
});

// ============================================================================
// calculateNetMinutes
// ============================================================================
describe("calculateNetMinutes", () => {
  it("returns null when no checkout", () => {
    expect(calculateNetMinutes("2024-03-15T08:00:00Z", null, 0)).toBeNull();
  });

  it("calculates net minutes correctly", () => {
    const result = calculateNetMinutes(
      "2024-03-15T08:00:00Z",
      "2024-03-15T16:00:00Z",
      30,
    );
    expect(result).toBe(450); // 480 - 30
  });

  it("returns 0 when break exceeds work time", () => {
    const result = calculateNetMinutes(
      "2024-03-15T08:00:00Z",
      "2024-03-15T08:10:00Z",
      60,
    );
    expect(result).toBe(0);
  });

  it("returns null for invalid check-in", () => {
    expect(
      calculateNetMinutes("invalid", "2024-03-15T16:00:00Z", 0),
    ).toBeNull();
  });

  it("returns null for invalid check-out", () => {
    expect(
      calculateNetMinutes("2024-03-15T08:00:00Z", "invalid", 0),
    ).toBeNull();
  });

  it("handles zero break", () => {
    const result = calculateNetMinutes(
      "2024-03-15T08:00:00Z",
      "2024-03-15T16:00:00Z",
      0,
    );
    expect(result).toBe(480);
  });
});
