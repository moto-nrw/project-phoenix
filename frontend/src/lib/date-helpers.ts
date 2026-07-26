/**
 * Date and time utility functions for formatting and grouping
 */

/** Matches a date-only string ("YYYY-MM-DD") as emitted for DATE columns. */
const ISO_DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

const BERLIN_DATE_PARTS_FORMATTER = new Intl.DateTimeFormat("en-US", {
  timeZone: "Europe/Berlin",
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
});

const CHAT_DAY_MONTH_FORMATTER = new Intl.DateTimeFormat("de-DE", {
  day: "2-digit",
  month: "2-digit",
  timeZone: "Europe/Berlin",
});

const CHAT_DATE_FORMATTER = new Intl.DateTimeFormat("de-DE", {
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  timeZone: "Europe/Berlin",
});

const CHAT_TIME_FORMATTER = new Intl.DateTimeFormat("de-DE", {
  hour: "2-digit",
  minute: "2-digit",
  timeZone: "Europe/Berlin",
});

/**
 * Serialize a Date to "YYYY-MM-DD" using LOCAL calendar fields.
 * NEVER derive it from toISOString() via split("T")[0] / slice(0, 10) —
 * toISOString() returns the UTC date, one day behind Berlin between 00:00 and 02:00
 * (enforced by oxlint rule date-safety/no-utc-date-extraction).
 */
export function toISODate(d: Date): string {
  const yyyy = d.getFullYear();
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return `${yyyy}-${mm}-${dd}`;
}

/**
 * Parse "YYYY-MM-DD" to a Date at LOCAL midnight.
 * `new Date("YYYY-MM-DD")` is UTC midnight (previous local day west of
 * UTC); the numeric constructor is local and DST-safe.
 */
export function parseISODate(s: string): Date {
  const y = Number(s.slice(0, 4));
  const m = Number(s.slice(5, 7));
  const d = Number(s.slice(8, 10));
  return new Date(y, m - 1, d);
}

/**
 * True when `s` is a real "YYYY-MM-DD" calendar date. The shape check alone
 * is not enough: parseISODate("2026-02-31") silently rolls over to March 3
 * (JS Date overflow arithmetic), so the round-trip through
 * toISODate(parseISODate(s)) rejects shape-valid but impossible dates. Use
 * this before feeding untrusted input (URL params, legacy deep links) into
 * parseISODate.
 */
export function isValidISODate(s: string): boolean {
  return ISO_DATE_RE.test(s) && toISODate(parseISODate(s)) === s;
}

/** Today's calendar date in the user's local timezone as "YYYY-MM-DD". */
export function todayISO(): string {
  return toISODate(new Date());
}

/**
 * Today's calendar date in the school's timezone (Europe/Berlin) as
 * "YYYY-MM-DD". Use this — not todayISO() — whenever the value is compared
 * against a backend DATE the server derives from `timezone.TodayDate()`
 * (always Berlin). A browser in another timezone can be a calendar day off
 * around midnight, which would otherwise send a week the backend rejects as
 * out of range. Built from Intl parts (not toISOString) so the date-safety
 * lint stays satisfied.
 */
export function berlinTodayISO(at: Date = new Date()): string {
  const parts = BERLIN_DATE_PARTS_FORMATTER.formatToParts(at);
  const get = (type: string) => parts.find((p) => p.type === type)?.value ?? "";
  return `${get("year")}-${get("month")}-${get("day")}`;
}

/**
 * Groups items by date, sorted in descending order (newest first)
 * @param items Array of items with timestamp properties
 * @param timestampKey The key to access the timestamp property
 * @returns Array of date groups with entries sorted by time
 */
export function groupByDate<T extends Record<string, unknown>>(
  items: T[],
  timestampKey: keyof T,
): Array<{ date: string; entries: T[] }> {
  const groups: Record<string, T[]> = {};

  items.forEach((item) => {
    const timestamp = item[timestampKey];
    if (typeof timestamp === "string") {
      const date = new Date(timestamp).toLocaleDateString("de-DE", {
        day: "2-digit",
        month: "2-digit",
        year: "numeric",
      });
      groups[date] ??= [];
      groups[date].push(item);
    }
  });

  return Object.keys(groups)
    .sort((a, b) => new Date(b).getTime() - new Date(a).getTime())
    .map((date) => ({
      date,
      entries: (groups[date] ?? []).sort((a, b) => {
        const timeA = a[timestampKey];
        const timeB = b[timestampKey];
        if (typeof timeA === "string" && typeof timeB === "string") {
          return new Date(timeA).getTime() - new Date(timeB).getTime();
        }
        return 0;
      }),
    }));
}

/**
 * Format a date string to German locale format
 * @param dateString ISO date string
 * @param includeWeekday Whether to include the weekday in the format
 * @returns Formatted date string (e.g., "15.12.2023" or "Freitag, 15. Dezember 2023")
 */
export function formatDate(dateString: string, includeWeekday = false): string {
  const date = ISO_DATE_RE.test(dateString)
    ? parseISODate(dateString)
    : new Date(dateString);
  if (includeWeekday) {
    return date.toLocaleDateString("de-DE", {
      weekday: "long",
      year: "numeric",
      month: "long",
      day: "numeric",
    });
  }
  return date.toLocaleDateString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

/**
 * Format a time string to German locale format
 * @param dateString ISO date string
 * @returns Formatted time string (e.g., "14:30")
 */
export function formatTime(dateString: string): string {
  return new Date(dateString).toLocaleTimeString("de-DE", {
    hour: "2-digit",
    minute: "2-digit",
  });
}

/**
 * Compact chat timestamp ("12.03., 14:30") for message bubbles — day/month plus
 * time, no year. Single source for the parent-OGS chat bubbles (chat-bubble,
 * ogs-conversation). Invalid ISO falls back to the raw input rather than
 * throwing, so a malformed timestamp never blanks a whole message list. Both
 * portions use the school timezone so the calendar date and clock cannot
 * disagree for guardians viewing from another timezone.
 */
export function formatChatTime(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  const dayMonth = CHAT_DAY_MONTH_FORMATTER.format(date);
  return `${dayMonth}, ${CHAT_TIME_FORMATTER.format(date)}`;
}

/**
 * Full chat timestamp with year ("12.03.2026, 14:30") for the message list and
 * thread headers. Returns "" for missing input; invalid ISO falls back to the
 * raw input. Single source for the staff inbox/thread pages and the parents
 * messages list. Both portions use the school timezone so full and compact
 * chat timestamps cannot disagree for viewers outside Europe/Berlin.
 */
export function formatChatDateTime(iso: string | undefined): string {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return `${CHAT_DATE_FORMATTER.format(date)}, ${CHAT_TIME_FORMATTER.format(date)}`;
}

/**
 * Calculate duration between two timestamps in minutes
 * @param startTime ISO date string
 * @param endTime ISO date string or null
 * @returns Duration in minutes, or null if endTime is not provided
 */
export function calculateDuration(
  startTime: string,
  endTime: string | null,
): number | null {
  if (!endTime) return null;
  const start = new Date(startTime);
  const end = new Date(endTime);
  return Math.round((end.getTime() - start.getTime()) / (1000 * 60));
}

/**
 * Format duration in minutes to human-readable string
 * @param minutes Duration in minutes, or null for active sessions
 * @returns Formatted duration string (e.g., "2 Std. 30 Min.", "45 Min.", or "Aktiv")
 */
export function formatDuration(minutes: number | null): string {
  if (minutes === null) return "Aktiv";
  if (minutes <= 0) return "< 1 Min.";

  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;

  if (hours > 0) {
    const minPart = mins > 0 ? `${mins} Min.` : "";
    return `${hours} Std. ${minPart}`.trim();
  }
  return `${mins} Min.`;
}

/**
 * Get the start date for a given time range filter
 * @param timeRange The time range filter option
 * @param referenceDate Optional reference date (defaults to now)
 * @returns Start date at midnight for the filter range
 */
export function getStartDateForTimeRange(
  timeRange: string,
  referenceDate: Date = new Date(),
): Date {
  const now = referenceDate;
  let startDate: Date;

  switch (timeRange) {
    case "today":
      startDate = new Date(now.getFullYear(), now.getMonth(), now.getDate());
      break;
    case "week": {
      const dayOfWeek = now.getDay();
      const daysFromMonday = dayOfWeek === 0 ? 6 : dayOfWeek - 1;
      startDate = new Date(
        now.getFullYear(),
        now.getMonth(),
        now.getDate() - daysFromMonday,
      );
      break;
    }
    case "month":
      startDate = new Date(now.getFullYear(), now.getMonth(), 1);
      break;
    case "7days":
    default:
      startDate = new Date(
        now.getFullYear(),
        now.getMonth(),
        now.getDate() - 6,
      );
  }

  startDate.setHours(0, 0, 0, 0);
  return startDate;
}
