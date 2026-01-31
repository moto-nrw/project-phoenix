import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import type {
  BackendPickupSchedule,
  BackendPickupException,
  BackendPickupData,
  PickupSchedule,
  PickupException,
} from "./pickup-schedule-helpers";
import {
  mapPickupScheduleResponse,
  mapPickupExceptionResponse,
  mapPickupDataResponse,
  mapPickupScheduleFormToBackend,
  mapBulkPickupScheduleFormToBackend,
  mapPickupExceptionFormToBackend,
  WEEKDAYS,
  getWeekdayLabel,
  getWeekdayShortLabel,
  formatPickupTime,
  formatExceptionDate,
  isDateInPast,
  getScheduleForWeekday,
  createEmptyWeeklySchedule,
  mergeSchedulesWithTemplate,
  getWeekStart,
  getWeekEnd,
  getWeekDays,
  formatShortDate,
  formatWeekRange,
  isSameDay,
  getWeekdayFromDate,
  getCalendarWeek,
  formatDateISO,
  getDayData,
} from "./pickup-schedule-helpers";

// Sample backend data for testing
const sampleBackendSchedule: BackendPickupSchedule = {
  id: 1,
  student_id: 100,
  weekday: 1,
  weekday_name: "Montag",
  pickup_time: "14:30",
  notes: "Bus line 5",
  created_by: 5,
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T12:00:00Z",
};

const sampleBackendException: BackendPickupException = {
  id: 1,
  student_id: 100,
  exception_date: "2024-01-15",
  pickup_time: "15:00",
  reason: "Doctor appointment",
  created_by: 5,
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T12:00:00Z",
};

describe("WEEKDAYS constant", () => {
  it("contains 5 weekdays", () => {
    expect(WEEKDAYS).toHaveLength(5);
  });

  it("has correct weekday values (1-5)", () => {
    expect(WEEKDAYS.map((w) => w.value)).toEqual([1, 2, 3, 4, 5]);
  });

  it("has German labels", () => {
    expect(WEEKDAYS[0]?.label).toBe("Montag");
    expect(WEEKDAYS[4]?.label).toBe("Freitag");
  });

  it("has short labels", () => {
    expect(WEEKDAYS[0]?.shortLabel).toBe("Mo");
    expect(WEEKDAYS[4]?.shortLabel).toBe("Fr");
  });
});

describe("mapPickupScheduleResponse", () => {
  it("maps backend schedule to frontend format", () => {
    const result = mapPickupScheduleResponse(sampleBackendSchedule);

    expect(result).toEqual({
      id: "1",
      studentId: "100",
      weekday: 1,
      weekdayName: "Montag",
      pickupTime: "14:30",
      notes: "Bus line 5",
      createdBy: "5",
      createdAt: "2024-01-15T10:00:00Z",
      updatedAt: "2024-01-15T12:00:00Z",
    });
  });

  it("converts all numeric IDs to strings", () => {
    const result = mapPickupScheduleResponse(sampleBackendSchedule);

    expect(typeof result.id).toBe("string");
    expect(typeof result.studentId).toBe("string");
    expect(typeof result.createdBy).toBe("string");
  });

  it("handles undefined notes", () => {
    const scheduleWithoutNotes = { ...sampleBackendSchedule, notes: undefined };
    const result = mapPickupScheduleResponse(scheduleWithoutNotes);

    expect(result.notes).toBeUndefined();
  });
});

describe("mapPickupExceptionResponse", () => {
  it("maps backend exception to frontend format", () => {
    const result = mapPickupExceptionResponse(sampleBackendException);

    expect(result).toEqual({
      id: "1",
      studentId: "100",
      exceptionDate: "2024-01-15",
      pickupTime: "15:00",
      reason: "Doctor appointment",
      createdBy: "5",
      createdAt: "2024-01-15T10:00:00Z",
      updatedAt: "2024-01-15T12:00:00Z",
    });
  });

  it("handles undefined pickup_time (absent)", () => {
    const absentException = {
      ...sampleBackendException,
      pickup_time: undefined,
    };
    const result = mapPickupExceptionResponse(absentException);

    expect(result.pickupTime).toBeUndefined();
  });
});

describe("mapPickupDataResponse", () => {
  it("maps combined backend data to frontend format", () => {
    const backendData: BackendPickupData = {
      schedules: [sampleBackendSchedule],
      exceptions: [sampleBackendException],
      notes: [],
    };

    const result = mapPickupDataResponse(backendData);

    expect(result.schedules).toHaveLength(1);
    expect(result.exceptions).toHaveLength(1);
    expect(result.schedules[0]?.id).toBe("1");
    expect(result.exceptions[0]?.id).toBe("1");
  });

  it("handles empty arrays", () => {
    const backendData: BackendPickupData = {
      schedules: [],
      exceptions: [],
      notes: [],
    };

    const result = mapPickupDataResponse(backendData);

    expect(result.schedules).toEqual([]);
    expect(result.exceptions).toEqual([]);
  });

  it("handles null/undefined arrays", () => {
    const backendData = {
      schedules: null,
      exceptions: null,
    } as unknown as BackendPickupData;

    const result = mapPickupDataResponse(backendData);

    expect(result.schedules).toEqual([]);
    expect(result.exceptions).toEqual([]);
  });
});

describe("mapPickupScheduleFormToBackend", () => {
  it("maps form data to backend request format", () => {
    const formData = {
      weekday: 1,
      pickupTime: "15:00",
      notes: "Test notes",
    };

    const result = mapPickupScheduleFormToBackend(formData);

    expect(result).toEqual({
      weekday: 1,
      pickup_time: "15:00",
      notes: "Test notes",
    });
  });

  it("handles undefined notes", () => {
    const formData = {
      weekday: 2,
      pickupTime: "16:00",
    };

    const result = mapPickupScheduleFormToBackend(formData);

    expect(result.notes).toBeUndefined();
  });
});

describe("mapBulkPickupScheduleFormToBackend", () => {
  it("maps bulk form data to backend request format", () => {
    const formData = {
      schedules: [
        { weekday: 1, pickupTime: "15:00", notes: "Monday notes" },
        { weekday: 3, pickupTime: "14:30" },
      ],
    };

    const result = mapBulkPickupScheduleFormToBackend(formData);

    expect(result.schedules).toHaveLength(2);
    expect(result.schedules[0]).toEqual({
      weekday: 1,
      pickup_time: "15:00",
      notes: "Monday notes",
    });
    expect(result.schedules[1]).toEqual({
      weekday: 3,
      pickup_time: "14:30",
      notes: undefined,
    });
  });

  it("handles empty schedules array", () => {
    const formData = { schedules: [] };
    const result = mapBulkPickupScheduleFormToBackend(formData);

    expect(result.schedules).toEqual([]);
  });
});

describe("mapPickupExceptionFormToBackend", () => {
  it("maps exception form data to backend request format", () => {
    const formData = {
      exceptionDate: "2024-01-20",
      pickupTime: "16:00",
      reason: "Dentist appointment",
    };

    const result = mapPickupExceptionFormToBackend(formData);

    expect(result).toEqual({
      exception_date: "2024-01-20",
      pickup_time: "16:00",
      reason: "Dentist appointment",
    });
  });

  it("handles undefined pickup time (absent)", () => {
    const formData = {
      exceptionDate: "2024-01-20",
      reason: "Sick day",
    };

    const result = mapPickupExceptionFormToBackend(formData);

    expect(result.pickup_time).toBeUndefined();
  });
});

describe("getWeekdayLabel", () => {
  it("returns correct German label for valid weekdays", () => {
    expect(getWeekdayLabel(1)).toBe("Montag");
    expect(getWeekdayLabel(2)).toBe("Dienstag");
    expect(getWeekdayLabel(3)).toBe("Mittwoch");
    expect(getWeekdayLabel(4)).toBe("Donnerstag");
    expect(getWeekdayLabel(5)).toBe("Freitag");
  });

  it("returns fallback for invalid weekday", () => {
    expect(getWeekdayLabel(0)).toBe("Tag 0");
    expect(getWeekdayLabel(6)).toBe("Tag 6");
    expect(getWeekdayLabel(10)).toBe("Tag 10");
  });
});

describe("getWeekdayShortLabel", () => {
  it("returns correct short labels for valid weekdays", () => {
    expect(getWeekdayShortLabel(1)).toBe("Mo");
    expect(getWeekdayShortLabel(2)).toBe("Di");
    expect(getWeekdayShortLabel(3)).toBe("Mi");
    expect(getWeekdayShortLabel(4)).toBe("Do");
    expect(getWeekdayShortLabel(5)).toBe("Fr");
  });

  it("returns fallback for invalid weekday", () => {
    expect(getWeekdayShortLabel(0)).toBe("T0");
    expect(getWeekdayShortLabel(6)).toBe("T6");
  });
});

describe("formatPickupTime", () => {
  it("returns HH:MM format unchanged", () => {
    expect(formatPickupTime("14:30")).toBe("14:30");
    expect(formatPickupTime("08:00")).toBe("08:00");
  });

  it("strips seconds from HH:MM:SS format", () => {
    expect(formatPickupTime("14:30:00")).toBe("14:30");
    expect(formatPickupTime("08:00:59")).toBe("08:00");
  });

  it("handles short time strings", () => {
    expect(formatPickupTime("14:3")).toBe("14:3");
    expect(formatPickupTime("8:00")).toBe("8:00");
  });
});

describe("formatExceptionDate", () => {
  it("formats date in German format", () => {
    const result = formatExceptionDate("2024-01-15");
    // Format: "Mo., 15.01.2024" or similar depending on locale
    expect(result).toContain("15");
    expect(result).toContain("01");
    expect(result).toContain("2024");
  });
});

describe("isDateInPast", () => {
  it("returns true for past dates", () => {
    expect(isDateInPast("2020-01-01")).toBe(true);
    expect(isDateInPast("1999-12-31")).toBe(true);
  });

  it("returns false for future dates", () => {
    expect(isDateInPast("2099-01-01")).toBe(false);
    expect(isDateInPast("2050-12-31")).toBe(false);
  });
});

describe("getScheduleForWeekday", () => {
  const schedules: PickupSchedule[] = [
    {
      id: "1",
      studentId: "100",
      weekday: 1,
      weekdayName: "Montag",
      pickupTime: "14:30",
      createdBy: "5",
      createdAt: "2024-01-15T10:00:00Z",
      updatedAt: "2024-01-15T12:00:00Z",
    },
    {
      id: "2",
      studentId: "100",
      weekday: 3,
      weekdayName: "Mittwoch",
      pickupTime: "15:00",
      createdBy: "5",
      createdAt: "2024-01-15T10:00:00Z",
      updatedAt: "2024-01-15T12:00:00Z",
    },
  ];

  it("finds schedule for existing weekday", () => {
    const result = getScheduleForWeekday(schedules, 1);
    expect(result?.id).toBe("1");

    const result2 = getScheduleForWeekday(schedules, 3);
    expect(result2?.id).toBe("2");
  });

  it("returns undefined for non-existent weekday", () => {
    const result = getScheduleForWeekday(schedules, 2);
    expect(result).toBeUndefined();
  });

  it("returns undefined for empty array", () => {
    const result = getScheduleForWeekday([], 1);
    expect(result).toBeUndefined();
  });
});

describe("createEmptyWeeklySchedule", () => {
  it("creates 5 empty schedule entries", () => {
    const result = createEmptyWeeklySchedule();

    expect(result).toHaveLength(5);
  });

  it("has correct weekday values", () => {
    const result = createEmptyWeeklySchedule();

    expect(result.map((s) => s.weekday)).toEqual([1, 2, 3, 4, 5]);
  });

  it("has empty pickup times", () => {
    const result = createEmptyWeeklySchedule();

    result.forEach((schedule) => {
      expect(schedule.pickupTime).toBe("");
      expect(schedule.notes).toBeUndefined();
    });
  });
});

describe("mergeSchedulesWithTemplate", () => {
  const existingSchedules: PickupSchedule[] = [
    {
      id: "1",
      studentId: "100",
      weekday: 1,
      weekdayName: "Montag",
      pickupTime: "14:30:00",
      notes: "Monday notes",
      createdBy: "5",
      createdAt: "2024-01-15T10:00:00Z",
      updatedAt: "2024-01-15T12:00:00Z",
    },
    {
      id: "2",
      studentId: "100",
      weekday: 3,
      weekdayName: "Mittwoch",
      pickupTime: "15:00",
      createdBy: "5",
      createdAt: "2024-01-15T10:00:00Z",
      updatedAt: "2024-01-15T12:00:00Z",
    },
  ];

  it("merges existing schedules with empty slots", () => {
    const result = mergeSchedulesWithTemplate(existingSchedules);

    expect(result).toHaveLength(5);
    expect(result[0]?.pickupTime).toBe("14:30"); // Formatted
    expect(result[1]?.pickupTime).toBe(""); // Empty (Tuesday)
    expect(result[2]?.pickupTime).toBe("15:00");
    expect(result[3]?.pickupTime).toBe(""); // Empty (Thursday)
    expect(result[4]?.pickupTime).toBe(""); // Empty (Friday)
  });

  it("preserves notes from existing schedules", () => {
    const result = mergeSchedulesWithTemplate(existingSchedules);

    expect(result[0]?.notes).toBe("Monday notes");
    expect(result[2]?.notes).toBeUndefined();
  });

  it("returns all empty for empty input", () => {
    const result = mergeSchedulesWithTemplate([]);

    result.forEach((schedule) => {
      expect(schedule.pickupTime).toBe("");
    });
  });
});

describe("getWeekStart", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns Monday of current week for offset 0", () => {
    vi.setSystemTime(new Date("2024-01-17T12:00:00Z")); // Wednesday

    const result = getWeekStart(0);

    expect(result.getDay()).toBe(1); // Monday
    expect(result.getDate()).toBe(15); // Jan 15, 2024
  });

  it("returns Monday of next week for offset 1", () => {
    vi.setSystemTime(new Date("2024-01-17T12:00:00Z")); // Wednesday

    const result = getWeekStart(1);

    expect(result.getDay()).toBe(1);
    expect(result.getDate()).toBe(22); // Jan 22, 2024
  });

  it("returns Monday of previous week for offset -1", () => {
    vi.setSystemTime(new Date("2024-01-17T12:00:00Z")); // Wednesday

    const result = getWeekStart(-1);

    expect(result.getDay()).toBe(1);
    expect(result.getDate()).toBe(8); // Jan 8, 2024
  });

  it("handles Sunday correctly", () => {
    vi.setSystemTime(new Date("2024-01-21T12:00:00Z")); // Sunday

    const result = getWeekStart(0);

    expect(result.getDay()).toBe(1);
    expect(result.getDate()).toBe(15); // Previous Monday
  });
});

describe("getWeekEnd", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns Friday of current week for offset 0", () => {
    vi.setSystemTime(new Date("2024-01-17T12:00:00Z")); // Wednesday

    const result = getWeekEnd(0);

    expect(result.getDay()).toBe(5); // Friday
    expect(result.getDate()).toBe(19); // Jan 19, 2024
  });

  it("returns Friday of next week for offset 1", () => {
    vi.setSystemTime(new Date("2024-01-17T12:00:00Z")); // Wednesday

    const result = getWeekEnd(1);

    expect(result.getDay()).toBe(5);
    expect(result.getDate()).toBe(26); // Jan 26, 2024
  });
});

describe("getWeekDays", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns 5 days (Mon-Fri)", () => {
    vi.setSystemTime(new Date("2024-01-17T12:00:00Z"));

    const result = getWeekDays(0);

    expect(result).toHaveLength(5);
  });

  it("returns consecutive weekdays", () => {
    vi.setSystemTime(new Date("2024-01-17T12:00:00Z"));

    const result = getWeekDays(0);

    expect(result[0]?.getDay()).toBe(1); // Monday
    expect(result[1]?.getDay()).toBe(2); // Tuesday
    expect(result[2]?.getDay()).toBe(3); // Wednesday
    expect(result[3]?.getDay()).toBe(4); // Thursday
    expect(result[4]?.getDay()).toBe(5); // Friday
  });
});

describe("formatShortDate", () => {
  it("formats date as DD.MM.", () => {
    const date = new Date("2024-01-15T12:00:00Z");
    const result = formatShortDate(date);

    expect(result).toBe("15.01.");
  });

  it("pads single digits", () => {
    const date = new Date("2024-03-05T12:00:00Z");
    const result = formatShortDate(date);

    expect(result).toBe("05.03.");
  });
});

describe("formatWeekRange", () => {
  it("formats week range correctly", () => {
    const start = new Date("2024-01-15T00:00:00Z");
    const end = new Date("2024-01-19T00:00:00Z");

    const result = formatWeekRange(start, end);

    expect(result).toBe("15.01.2024 - 19.01.2024");
  });

  it("handles cross-month ranges", () => {
    const start = new Date("2024-01-29T00:00:00Z");
    const end = new Date("2024-02-02T00:00:00Z");

    const result = formatWeekRange(start, end);

    expect(result).toBe("29.01.2024 - 02.02.2024");
  });
});

describe("isSameDay", () => {
  it("returns true for same day", () => {
    const date1 = new Date("2024-01-15T10:00:00Z");
    const date2 = new Date("2024-01-15T18:00:00Z");

    expect(isSameDay(date1, date2)).toBe(true);
  });

  it("returns false for different days", () => {
    const date1 = new Date("2024-01-15T10:00:00Z");
    const date2 = new Date("2024-01-16T10:00:00Z");

    expect(isSameDay(date1, date2)).toBe(false);
  });

  it("returns false for different months", () => {
    const date1 = new Date("2024-01-15T10:00:00Z");
    const date2 = new Date("2024-02-15T10:00:00Z");

    expect(isSameDay(date1, date2)).toBe(false);
  });
});

describe("getWeekdayFromDate", () => {
  it("returns 1 for Monday", () => {
    const monday = new Date("2024-01-15T12:00:00Z");
    expect(getWeekdayFromDate(monday)).toBe(1);
  });

  it("returns 5 for Friday", () => {
    const friday = new Date("2024-01-19T12:00:00Z");
    expect(getWeekdayFromDate(friday)).toBe(5);
  });

  it("returns null for Saturday", () => {
    const saturday = new Date("2024-01-20T12:00:00Z");
    expect(getWeekdayFromDate(saturday)).toBeNull();
  });

  it("returns null for Sunday", () => {
    const sunday = new Date("2024-01-21T12:00:00Z");
    expect(getWeekdayFromDate(sunday)).toBeNull();
  });
});

describe("getCalendarWeek", () => {
  it("returns week number for middle of year", () => {
    const date = new Date("2024-06-15T12:00:00Z");
    const result = getCalendarWeek(date);

    expect(result).toBeGreaterThan(0);
    expect(result).toBeLessThanOrEqual(53);
  });

  it("returns 1 for first week of year", () => {
    // January 4, 2024 is guaranteed to be in week 1
    const date = new Date("2024-01-04T12:00:00Z");
    const result = getCalendarWeek(date);

    expect(result).toBe(1);
  });

  it("returns consistent results for same week", () => {
    const monday = new Date("2024-01-15T12:00:00Z");
    const friday = new Date("2024-01-19T12:00:00Z");

    expect(getCalendarWeek(monday)).toBe(getCalendarWeek(friday));
  });
});

describe("formatDateISO", () => {
  it("formats date as YYYY-MM-DD", () => {
    const date = new Date("2024-01-15T12:00:00Z");
    const result = formatDateISO(date);

    expect(result).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });

  it("pads single-digit months", () => {
    const date = new Date("2024-03-15T12:00:00Z");
    const result = formatDateISO(date);

    expect(result).toContain("-03-");
  });

  it("pads single-digit days", () => {
    const date = new Date("2024-01-05T12:00:00Z");
    const result = formatDateISO(date);

    expect(result).toContain("-05");
  });
});

describe("getDayData", () => {
  const schedules: PickupSchedule[] = [
    {
      id: "1",
      studentId: "100",
      weekday: 1,
      weekdayName: "Montag",
      pickupTime: "14:30",
      notes: "Regular schedule",
      createdBy: "5",
      createdAt: "2024-01-15T10:00:00Z",
      updatedAt: "2024-01-15T12:00:00Z",
    },
  ];

  const exceptions: PickupException[] = [
    {
      id: "1",
      studentId: "100",
      exceptionDate: "2024-01-15",
      pickupTime: "15:00",
      reason: "Doctor appointment",
      createdBy: "5",
      createdAt: "2024-01-15T10:00:00Z",
      updatedAt: "2024-01-15T12:00:00Z",
    },
  ];

  it("returns base schedule when no exception", () => {
    const date = new Date("2024-01-22T12:00:00Z"); // Monday without exception
    const result = getDayData(date, schedules, [], false);

    expect(result.weekday).toBe(1);
    expect(result.baseSchedule?.pickupTime).toBe("14:30");
    expect(result.effectiveTime).toBe("14:30");
    expect(result.effectiveNotes).toBe("Regular schedule");
    expect(result.isException).toBe(false);
  });

  it("returns exception when present", () => {
    const date = new Date("2024-01-15T12:00:00Z");
    const result = getDayData(date, schedules, exceptions, false);

    expect(result.exception?.pickupTime).toBe("15:00");
    expect(result.effectiveTime).toBe("15:00");
    expect(result.effectiveNotes).toBe("Doctor appointment");
    expect(result.isException).toBe(true);
  });

  it("returns sick status when student is sick today", () => {
    // Use a fixed weekday (Monday) so the test doesn't fail on weekends
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2024-01-15T12:00:00")); // Monday
    try {
      const today = new Date();
      const result = getDayData(today, schedules, [], true);

      expect(result.showSick).toBe(true);
      expect(result.effectiveTime).toBeUndefined();
    } finally {
      vi.useRealTimers();
    }
  });

  it("does not show sick status for past days", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2024-01-16T12:00:00")); // Tuesday
    try {
      const yesterday = new Date("2024-01-15T12:00:00"); // Monday
      const result = getDayData(yesterday, schedules, [], true);

      expect(result.showSick).toBe(false);
    } finally {
      vi.useRealTimers();
    }
  });

  it("marks today correctly", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2024-01-15T12:00:00")); // Monday
    try {
      const today = new Date();
      const result = getDayData(today, schedules, [], false);

      expect(result.isToday).toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });

  it("handles no schedule for weekday", () => {
    const tuesday = new Date("2024-01-16T12:00:00Z");
    const result = getDayData(tuesday, schedules, [], false);

    expect(result.baseSchedule).toBeUndefined();
    expect(result.effectiveTime).toBeUndefined();
  });

  it("exception with no pickup time (absent)", () => {
    const absentException: PickupException = {
      id: "2",
      studentId: "100",
      exceptionDate: "2024-01-16",
      reason: "Student absent",
      createdBy: "5",
      createdAt: "2024-01-15T10:00:00Z",
      updatedAt: "2024-01-15T12:00:00Z",
    };

    const date = new Date("2024-01-16T12:00:00Z");
    const result = getDayData(date, schedules, [absentException], false);

    expect(result.exception?.pickupTime).toBeUndefined();
    expect(result.effectiveTime).toBeUndefined();
    expect(result.isException).toBe(true);
  });
});
