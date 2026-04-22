import { describe, it, expect } from "vitest";
import {
  mapAttendanceHistoryResponse,
  formatTime,
  formatDate,
  formatDuration,
  type BackendAttendanceHistoryResponse,
} from "./attendance-history-helpers";

describe("mapAttendanceHistoryResponse", () => {
  const raw: BackendAttendanceHistoryResponse = {
    student_id: "42",
    days: [
      {
        date: "2026-04-06",
        attendance: {
          check_in_time: "2026-04-06T08:00:00Z",
          check_out_time: "2026-04-06T15:30:00Z",
          duration_minutes: 450,
          checked_in_by: 11,
          checked_out_by: 11,
          device_id: 3,
        },
        room_detail_available: true,
        visits: [
          {
            room_id: 5,
            room_name: "Gruppenraum A",
            entry_time: "2026-04-06T08:10:00Z",
            exit_time: "2026-04-06T10:30:00Z",
            duration_minutes: 140,
          },
        ],
      },
      {
        date: "2026-03-20",
        attendance: {
          check_in_time: "2026-03-20T08:00:00Z",
          check_out_time: null,
          duration_minutes: null,
          checked_in_by: 11,
          device_id: 3,
        },
        room_detail_available: false,
        visits: null,
      },
    ],
    range: {
      start: "2026-03-07T00:00:00Z",
      end: "2026-04-06T23:59:59Z",
    },
    clamped: false,
    caps: { attendance_days: 30, room_detail_days: 7 },
  };

  it("maps top-level fields correctly", () => {
    const result = mapAttendanceHistoryResponse(raw);
    expect(result.studentId).toBe("42");
    expect(result.clamped).toBe(false);
    expect(result.caps.attendanceDays).toBe(30);
    expect(result.caps.roomDetailDays).toBe(7);
    expect(result.range.start).toBeInstanceOf(Date);
    expect(result.range.end).toBeInstanceOf(Date);
  });

  it("maps days with attendance and visits", () => {
    const result = mapAttendanceHistoryResponse(raw);
    expect(result.days).toHaveLength(2);

    const day1 = result.days[0]!;
    expect(day1.date).toBe("2026-04-06");
    expect(day1.roomDetailAvailable).toBe(true);
    expect(day1.attendance).not.toBeNull();
    expect(day1.attendance!.checkInTime).toBeInstanceOf(Date);
    expect(day1.attendance!.checkOutTime).toBeInstanceOf(Date);
    expect(day1.attendance!.durationMinutes).toBe(450);
    expect(day1.attendance!.checkedInBy).toBe(11);
    expect(day1.attendance!.deviceId).toBe(3);

    expect(day1.visits).toHaveLength(1);
    expect(day1.visits[0]!.roomName).toBe("Gruppenraum A");
    expect(day1.visits[0]!.roomId).toBe(5);
    expect(day1.visits[0]!.entryTime).toBeInstanceOf(Date);
    expect(day1.visits[0]!.exitTime).toBeInstanceOf(Date);
    expect(day1.visits[0]!.durationMinutes).toBe(140);
  });

  it("maps days without room detail and null visits", () => {
    const result = mapAttendanceHistoryResponse(raw);
    const day2 = result.days[1]!;
    expect(day2.roomDetailAvailable).toBe(false);
    expect(day2.visits).toEqual([]);
    expect(day2.attendance!.checkOutTime).toBeNull();
    expect(day2.attendance!.durationMinutes).toBeNull();
  });

  it("maps a web-originated attendance row with null device_id to null deviceId", () => {
    // Backend migration 1.15.41 made device_id nullable — web school-checkin
    // rows omit the device. The mapper must coalesce undefined/null to null
    // so consumers don't see a NaN-ish number.
    const webRaw: BackendAttendanceHistoryResponse = {
      ...raw,
      days: [
        {
          date: "2026-04-10",
          attendance: {
            check_in_time: "2026-04-10T09:00:00Z",
            check_out_time: null,
            duration_minutes: null,
            checked_in_by: 11,
            // device_id omitted — simulates `null` from the backend JSON
          },
          room_detail_available: false,
          visits: null,
        },
      ],
    };
    const result = mapAttendanceHistoryResponse(webRaw);
    expect(result.days[0]!.attendance!.deviceId).toBeNull();
  });
});

describe("formatDuration", () => {
  it("returns dash for null", () => {
    expect(formatDuration(null)).toBe("–");
  });

  it("formats minutes only", () => {
    expect(formatDuration(45)).toBe("45 min");
  });

  it("formats hours only", () => {
    expect(formatDuration(120)).toBe("2 h");
  });

  it("formats hours + minutes", () => {
    expect(formatDuration(150)).toBe("2 h 30 min");
  });

  it("formats zero", () => {
    expect(formatDuration(0)).toBe("0 min");
  });
});

describe("formatDate", () => {
  it("formats a date key to German locale", () => {
    const result = formatDate("2026-04-06");
    // Should contain the German day name and month
    expect(result).toContain("2026");
    expect(result).toContain("April");
  });
});

describe("formatTime", () => {
  it("formats a Date to HH:MM", () => {
    const d = new Date("2026-04-06T08:15:00Z");
    const result = formatTime(d);
    // Should be "HH:MM" format (exact value depends on test runner TZ)
    expect(result).toMatch(/^\d{2}:\d{2}$/);
  });
});
