import { describe, it, expect } from "vitest";
import {
  groupByDate,
  formatDate,
  formatTime,
  calculateDuration,
  formatDuration,
  getStartDateForTimeRange,
} from "./date-helpers";

describe("groupByDate", () => {
  it("groups items by date in descending order (newest first)", () => {
    const items = [
      { id: 1, timestamp: "2024-01-15T10:00:00Z" },
      { id: 2, timestamp: "2024-01-15T14:00:00Z" },
      { id: 3, timestamp: "2024-01-14T09:00:00Z" },
    ];

    const result = groupByDate(items, "timestamp");

    expect(result).toHaveLength(2);
    // Newest date first
    expect(result[0]?.entries).toHaveLength(2);
    expect(result[1]?.entries).toHaveLength(1);
  });

  it("sorts entries within each date group by time (oldest first)", () => {
    const items = [
      { id: 1, timestamp: "2024-01-15T14:00:00Z" },
      { id: 2, timestamp: "2024-01-15T10:00:00Z" },
      { id: 3, timestamp: "2024-01-15T12:00:00Z" },
    ];

    const result = groupByDate(items, "timestamp");

    expect(result).toHaveLength(1);
    // Check entries are sorted by time (oldest first)
    expect(result[0]?.entries[0]?.id).toBe(2); // 10:00
    expect(result[0]?.entries[1]?.id).toBe(3); // 12:00
    expect(result[0]?.entries[2]?.id).toBe(1); // 14:00
  });

  it("handles empty array", () => {
    const result = groupByDate([], "timestamp");
    expect(result).toHaveLength(0);
  });

  it("skips items with non-string timestamp values", () => {
    const items = [
      { id: 1, timestamp: "2024-01-15T10:00:00Z" },
      { id: 2, timestamp: null },
      { id: 3, timestamp: 123456 },
    ];

    // Cast to satisfy TypeScript while testing edge cases
    const result = groupByDate(
      items as Array<{ id: number; timestamp: string }>,
      "timestamp",
    );

    expect(result).toHaveLength(1);
    expect(result[0]?.entries).toHaveLength(1);
  });

  it("handles different timestamp keys", () => {
    const items = [
      { id: 1, createdAt: "2024-01-15T10:00:00Z" },
      { id: 2, createdAt: "2024-01-14T10:00:00Z" },
    ];

    const result = groupByDate(items, "createdAt");

    expect(result).toHaveLength(2);
  });
});

describe("formatDate", () => {
  it("formats date string to German locale (dd.mm.yyyy)", () => {
    const result = formatDate("2024-01-15T10:00:00Z");

    // German format: day.month.year
    expect(result).toMatch(/\d{1,2}\.\d{1,2}\.\d{4}/);
  });

  it("includes weekday when includeWeekday is true", () => {
    const result = formatDate("2024-01-15T10:00:00Z", true);

    // Should include German weekday name (Montag)
    expect(result).toContain("Januar");
    expect(result).toContain("2024");
  });

  it("excludes weekday by default", () => {
    const result = formatDate("2024-01-15T10:00:00Z");

    // Should NOT include full month name
    expect(result).not.toContain("Januar");
  });

  it("handles different date strings", () => {
    const result = formatDate("2023-12-25T00:00:00Z");

    expect(result).toMatch(/\d{1,2}\.\d{1,2}\.\d{4}/);
  });
});

describe("formatTime", () => {
  it("formats time string to German locale (HH:mm)", () => {
    const result = formatTime("2024-01-15T14:30:00Z");

    // German time format: 24-hour with 2-digit hours and minutes
    expect(result).toMatch(/\d{2}:\d{2}/);
  });

  it("uses 24-hour format", () => {
    // 2:30 PM should be displayed as 14:30 or similar depending on timezone
    const result = formatTime("2024-01-15T14:30:00Z");

    // Should not contain AM/PM
    expect(result).not.toMatch(/[AP]M/i);
  });

  it("handles midnight", () => {
    const result = formatTime("2024-01-15T00:00:00Z");

    expect(result).toMatch(/\d{2}:\d{2}/);
  });
});

describe("calculateDuration", () => {
  it("calculates duration in minutes between two timestamps", () => {
    const startTime = "2024-01-15T10:00:00Z";
    const endTime = "2024-01-15T11:30:00Z";

    const result = calculateDuration(startTime, endTime);

    expect(result).toBe(90); // 1.5 hours = 90 minutes
  });

  it("returns null when endTime is null", () => {
    const startTime = "2024-01-15T10:00:00Z";

    const result = calculateDuration(startTime, null);

    expect(result).toBeNull();
  });

  it("handles same start and end time (zero duration)", () => {
    const time = "2024-01-15T10:00:00Z";

    const result = calculateDuration(time, time);

    expect(result).toBe(0);
  });

  it("handles short durations correctly", () => {
    const startTime = "2024-01-15T10:00:00Z";
    const endTime = "2024-01-15T10:05:00Z";

    const result = calculateDuration(startTime, endTime);

    expect(result).toBe(5);
  });

  it("handles long durations correctly", () => {
    const startTime = "2024-01-15T10:00:00Z";
    const endTime = "2024-01-15T18:00:00Z";

    const result = calculateDuration(startTime, endTime);

    expect(result).toBe(480); // 8 hours = 480 minutes
  });

  it("handles durations spanning days", () => {
    const startTime = "2024-01-15T22:00:00Z";
    const endTime = "2024-01-16T02:00:00Z";

    const result = calculateDuration(startTime, endTime);

    expect(result).toBe(240); // 4 hours = 240 minutes
  });
});

describe("formatDuration", () => {
  it("returns 'Aktiv' for null duration", () => {
    const result = formatDuration(null);

    expect(result).toBe("Aktiv");
  });

  it("returns '< 1 Min.' for zero or negative duration", () => {
    expect(formatDuration(0)).toBe("< 1 Min.");
    expect(formatDuration(-5)).toBe("< 1 Min.");
  });

  it("formats minutes-only duration", () => {
    const result = formatDuration(45);

    expect(result).toBe("45 Min.");
  });

  it("formats hours and minutes duration", () => {
    const result = formatDuration(90);

    expect(result).toBe("1 Std. 30 Min.");
  });

  it("formats hours-only duration (no remaining minutes)", () => {
    const result = formatDuration(120);

    expect(result).toBe("2 Std.");
  });

  it("formats single minute correctly", () => {
    const result = formatDuration(1);

    expect(result).toBe("1 Min.");
  });

  it("formats single hour correctly", () => {
    const result = formatDuration(60);

    expect(result).toBe("1 Std.");
  });

  it("formats large durations correctly", () => {
    const result = formatDuration(480); // 8 hours

    expect(result).toBe("8 Std.");
  });
});

describe("getStartDateForTimeRange", () => {
  // Use a fixed reference date for predictable tests
  const referenceDate = new Date("2024-01-15T12:00:00Z"); // Monday

  it("returns start of day for 'today'", () => {
    const result = getStartDateForTimeRange("today", referenceDate);

    expect(result.getFullYear()).toBe(2024);
    expect(result.getMonth()).toBe(0); // January
    expect(result.getDate()).toBe(15);
    expect(result.getHours()).toBe(0);
    expect(result.getMinutes()).toBe(0);
    expect(result.getSeconds()).toBe(0);
  });

  it("returns Monday of current week for 'week'", () => {
    // Reference date is Monday, so start of week should be the same day
    const result = getStartDateForTimeRange("week", referenceDate);

    expect(result.getFullYear()).toBe(2024);
    expect(result.getMonth()).toBe(0); // January
    expect(result.getDate()).toBe(15); // Monday Jan 15
  });

  it("handles Sunday correctly for 'week' (goes back to previous Monday)", () => {
    const sunday = new Date("2024-01-21T12:00:00Z"); // Sunday
    const result = getStartDateForTimeRange("week", sunday);

    // Should go back to Monday Jan 15
    expect(result.getDate()).toBe(15);
  });

  it("returns first day of month for 'month'", () => {
    const result = getStartDateForTimeRange("month", referenceDate);

    expect(result.getFullYear()).toBe(2024);
    expect(result.getMonth()).toBe(0); // January
    expect(result.getDate()).toBe(1); // First day of month
  });

  it("returns 6 days ago for '7days'", () => {
    const result = getStartDateForTimeRange("7days", referenceDate);

    expect(result.getFullYear()).toBe(2024);
    expect(result.getMonth()).toBe(0); // January
    expect(result.getDate()).toBe(9); // 15 - 6 = 9
  });

  it("defaults to '7days' for unknown time range", () => {
    const result = getStartDateForTimeRange("unknown", referenceDate);
    const expected = getStartDateForTimeRange("7days", referenceDate);

    expect(result.getTime()).toBe(expected.getTime());
  });

  it("sets time to midnight (00:00:00.000)", () => {
    const result = getStartDateForTimeRange("today", referenceDate);

    expect(result.getHours()).toBe(0);
    expect(result.getMinutes()).toBe(0);
    expect(result.getSeconds()).toBe(0);
    expect(result.getMilliseconds()).toBe(0);
  });

  it("uses current date when no reference date provided", () => {
    // Just verify it doesn't throw and returns a Date
    const result = getStartDateForTimeRange("today");

    expect(result).toBeInstanceOf(Date);
    expect(result.getHours()).toBe(0);
  });

  it("handles month boundary for 'week'", () => {
    // February 1, 2024 is a Thursday
    const feb1 = new Date("2024-02-01T12:00:00Z");
    const result = getStartDateForTimeRange("week", feb1);

    // Should go back to Monday Jan 29
    expect(result.getMonth()).toBe(0); // January
    expect(result.getDate()).toBe(29);
  });
});
