import { describe, it, expect, afterEach } from "vitest";
import {
  berlinDayFromISO,
  groupByDate,
  formatDate,
  formatTime,
  calculateDuration,
  formatDuration,
  getStartDateForTimeRange,
  toISODate,
  parseISODate,
  isValidISODate,
  todayISO,
  berlinTodayISO,
  berlinDayStart,
  endOfBerlinDayISO,
  formatChatTime,
  formatChatDateTime,
  formatChatClockTime,
  formatBerlinDate,
} from "./date-helpers";

describe("toISODate", () => {
  it("serializes a Date using LOCAL calendar fields", () => {
    const d = new Date(2026, 5, 10); // June 10, 2026 local midnight
    expect(toISODate(d)).toBe("2026-06-10");
  });

  it("keeps the local calendar date at 23:30 local time", () => {
    // 23:30 local — toISOString() would already be on the next UTC day
    // east of UTC (Berlin), but the LOCAL calendar date must not change.
    const d = new Date(2026, 0, 15, 23, 30);
    expect(toISODate(d)).toBe("2026-01-15");
  });

  it("pads single-digit month and day", () => {
    const d = new Date(2026, 2, 5);
    expect(toISODate(d)).toBe("2026-03-05");
  });
});

describe("parseISODate", () => {
  it("parses to LOCAL midnight", () => {
    const d = parseISODate("2026-06-10");
    expect(d.getFullYear()).toBe(2026);
    expect(d.getMonth()).toBe(5);
    expect(d.getDate()).toBe(10);
    expect(d.getHours()).toBe(0);
    expect(d.getMinutes()).toBe(0);
  });

  it("roundtrips toISODate(parseISODate(s)) === s", () => {
    for (const s of ["2026-01-01", "2026-06-10", "2025-12-31", "2024-02-29"]) {
      expect(toISODate(parseISODate(s))).toBe(s);
    }
  });
});

describe("isValidISODate", () => {
  it("accepts real YYYY-MM-DD calendar dates", () => {
    for (const s of ["2026-01-01", "2026-06-10", "2024-02-29"]) {
      expect(isValidISODate(s)).toBe(true);
    }
  });

  it("rejects strings that are not shaped like an ISO date", () => {
    for (const s of ["foo", "", "2026-7-1", "2026-07-01T00:00:00Z", "<x>"]) {
      expect(isValidISODate(s)).toBe(false);
    }
  });

  it("rejects shape-valid but impossible dates (no silent rollover)", () => {
    // parseISODate("2026-02-31") rolls over to March 3 — the round-trip
    // check must catch that instead of accepting the rolled-over date.
    for (const s of ["2026-02-31", "2026-13-01", "2025-02-29", "2026-00-10"]) {
      expect(isValidISODate(s)).toBe(false);
    }
  });
});

describe("todayISO", () => {
  it("returns today's local calendar date as YYYY-MM-DD", () => {
    expect(todayISO()).toBe(toISODate(new Date()));
    expect(todayISO()).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });
});

describe("berlinTodayISO", () => {
  it("returns a well-formed YYYY-MM-DD string", () => {
    expect(berlinTodayISO()).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });

  it("matches today's date computed directly in the Europe/Berlin timezone", () => {
    const expected = new Intl.DateTimeFormat("en-CA", {
      timeZone: "Europe/Berlin",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).format(new Date()); // en-CA formats as YYYY-MM-DD
    expect(berlinTodayISO()).toBe(expected);
  });
});

describe("endOfBerlinDayISO", () => {
  it("uses the CEST end of day for a selected summer date", () => {
    expect(endOfBerlinDayISO(new Date(2026, 6, 20))).toBe(
      "2026-07-20T21:59:59.000Z",
    );
  });

  it("uses the CET end of day for a selected winter date", () => {
    expect(endOfBerlinDayISO(new Date(2026, 0, 20))).toBe(
      "2026-01-20T22:59:59.000Z",
    );
  });
});

describe("berlinDayStart", () => {
  it("returns Berlin midnight for a winter instant", () => {
    expect(berlinDayStart(new Date("2026-01-15T01:30:00Z")).toISOString()).toBe(
      "2026-01-14T23:00:00.000Z",
    );
  });

  it("returns Berlin midnight for a summer instant", () => {
    expect(berlinDayStart(new Date("2026-07-20T10:00:00Z")).toISOString()).toBe(
      "2026-07-19T22:00:00.000Z",
    );
  });

  // The night the clocks go forward is 23h long; subtracting 24h from the
  // following midnight would land an hour beside the real day boundary.
  it("hits the real midnight on the night DST starts", () => {
    expect(berlinDayStart(new Date("2026-03-29T12:00:00Z")).toISOString()).toBe(
      "2026-03-28T23:00:00.000Z",
    );
  });

  it("hits the real midnight on the night DST ends", () => {
    expect(berlinDayStart(new Date("2026-10-25T12:00:00Z")).toISOString()).toBe(
      "2026-10-24T22:00:00.000Z",
    );
  });
});

describe("berlinDayFromISO", () => {
  // The tests below run in a browser timezone east of Berlin, where a Berlin
  // end-of-day instant already falls on the next local calendar day.
  const withTimezone = (tz: string, run: () => void) => {
    const previous = process.env.TZ;
    process.env.TZ = tz;
    try {
      run();
    } finally {
      process.env.TZ = previous;
    }
  };

  afterEach(() => {
    expect(process.env.TZ).toBe("Europe/Berlin");
  });

  it("keeps the Berlin day of a summer cut-off east of Berlin", () => {
    withTimezone("Europe/Moscow", () => {
      // 23:59:59 CEST on 20.07. — 02:59 the next morning in Moscow
      const day = berlinDayFromISO("2026-07-20T21:59:59.000Z");
      expect(toISODate(day!)).toBe("2026-07-20");
    });
  });

  it("keeps the Berlin day of a winter cut-off east of Berlin", () => {
    withTimezone("Europe/Kyiv", () => {
      // 23:59:59 CET on 20.01. — 00:59 the next morning in Kyiv
      const day = berlinDayFromISO("2026-01-20T22:59:59.000Z");
      expect(toISODate(day!)).toBe("2026-01-20");
    });
  });

  it("round-trips through endOfBerlinDayISO without moving the day", () => {
    // Reopening an unchanged draft and saving it must not extend the cut-off.
    withTimezone("Asia/Tokyo", () => {
      const stored = "2026-07-20T21:59:59.000Z";
      expect(endOfBerlinDayISO(berlinDayFromISO(stored)!)).toBe(stored);
    });
  });

  it("returns null for an unparseable value", () => {
    expect(berlinDayFromISO("nicht-datum")).toBeNull();
  });
});

describe("formatBerlinDate", () => {
  // A cut-off written by endOfBerlinDayISO is 23:59:59 Berlin. East of Berlin
  // that instant already belongs to the next LOCAL day, so rendering it in the
  // viewer's timezone would advertise a deadline that has in fact passed.
  const withTimezone = (tz: string, run: () => void) => {
    const previous = process.env.TZ;
    process.env.TZ = tz;
    try {
      run();
    } finally {
      process.env.TZ = previous;
    }
  };

  afterEach(() => {
    expect(process.env.TZ).toBe("Europe/Berlin");
  });

  it("keeps the Berlin day of a summer cut-off east of Berlin", () => {
    withTimezone("Europe/Moscow", () => {
      expect(formatBerlinDate("2026-07-20T21:59:59.000Z")).toBe("20.07.2026");
    });
  });

  it("keeps the Berlin day of a winter cut-off east of Berlin", () => {
    withTimezone("Asia/Tokyo", () => {
      expect(formatBerlinDate("2026-01-20T22:59:59.000Z")).toBe("20.01.2026");
    });
  });

  it("keeps the Berlin day west of Berlin", () => {
    withTimezone("America/Los_Angeles", () => {
      expect(formatBerlinDate("2026-07-20T21:59:59.000Z")).toBe("20.07.2026");
    });
  });

  it("falls back to the raw input for an unparseable value", () => {
    expect(formatBerlinDate("nicht-datum")).toBe("nicht-datum");
  });

  it("uses the requested locale", () => {
    expect(formatBerlinDate("2026-07-20T21:59:59.000Z", "en-GB")).toBe(
      "20/07/2026",
    );
  });
});

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

  it("renders a date-only string ('YYYY-MM-DD') as the same German calendar date", () => {
    // Date-only input must route through parseISODate (local midnight),
    // never through new Date("YYYY-MM-DD") (UTC midnight).
    expect(formatDate("2026-06-10")).toBe("10.06.2026");
  });

  it("renders a date-only string with weekday without day shift", () => {
    const result = formatDate("2026-06-10", true);

    expect(result).toContain("Mittwoch");
    expect(result).toContain("10. Juni 2026");
  });

  it("uses the requested locale for parent-facing dates", () => {
    expect(formatDate("2026-06-10", false, "en-US")).toBe("06/10/2026");
    expect(formatDate("2026-06-10", true, "en-US")).toContain(
      "Wednesday, June 10, 2026",
    );
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

describe("formatChatTime", () => {
  it("returns 'dd.MM., HH:mm' format for a valid ISO timestamp", () => {
    const result = formatChatTime("2024-01-15T13:30:00Z");
    expect(result).toBe("15.01., 14:30");
  });

  it("formats the date and clock in Berlin across midnight", () => {
    expect(formatChatTime("2026-01-01T23:30:00Z")).toBe("02.01., 00:30");
  });

  it("returns the raw input string for an invalid ISO (never throws)", () => {
    expect(formatChatTime("not-a-date")).toBe("not-a-date");
  });

  it("returns the raw input for an empty string", () => {
    expect(formatChatTime("")).toBe("");
  });

  it("includes both day/month and clock parts separated by ', '", () => {
    const result = formatChatTime("2026-06-10T08:05:00Z");
    expect(result).toContain(", ");
    const parts = result.split(", ");
    expect(parts).toHaveLength(2);
    // day.month. part
    expect(parts[0]).toMatch(/^\d{2}\.\d{2}\.$/);
    // HH:MM part
    expect(parts[1]).toMatch(/^\d{2}:\d{2}$/);
  });

  it("uses the requested locale", () => {
    expect(formatChatTime("2026-06-10T08:05:00Z", "en-GB")).toBe(
      "10/06, 10:05",
    );
  });
});

describe("formatChatDateTime", () => {
  it("returns empty string for undefined", () => {
    expect(formatChatDateTime(undefined)).toBe("");
  });

  it("returns empty string for empty string", () => {
    expect(formatChatDateTime("")).toBe("");
  });

  it("returns the raw input for an invalid ISO string", () => {
    expect(formatChatDateTime("garbage")).toBe("garbage");
  });

  it("formats the date and clock in Berlin for a viewer in another timezone", () => {
    const previousTZ = process.env.TZ;
    process.env.TZ = "America/Los_Angeles";
    try {
      expect(formatChatDateTime("2026-01-01T23:30:00Z")).toBe(
        "02.01.2026, 00:30",
      );
    } finally {
      if (previousTZ === undefined) delete process.env.TZ;
      else process.env.TZ = previousTZ;
    }
  });

  it("handles a date-only string ('YYYY-MM-DD') without throwing", () => {
    // date-only strings are valid ISO and should produce a formatted result
    const result = formatChatDateTime("2026-06-10");
    expect(typeof result).toBe("string");
    expect(result.length).toBeGreaterThan(0);
  });

  it("uses the requested locale", () => {
    expect(formatChatDateTime("2026-06-10T08:05:00Z", "en-GB")).toBe(
      "10/06/2026, 10:05",
    );
  });
});

describe("formatChatClockTime", () => {
  it("formats only the Berlin clock time for compact chat lists", () => {
    expect(formatChatClockTime("2026-01-01T23:30:00Z")).toBe("00:30");
  });

  it("returns the raw input for an invalid timestamp", () => {
    expect(formatChatClockTime("garbage")).toBe("garbage");
  });

  it("uses the requested locale", () => {
    expect(formatChatClockTime("2026-06-10T20:05:00Z", "en-US")).toBe(
      "10:05 PM",
    );
  });
});
