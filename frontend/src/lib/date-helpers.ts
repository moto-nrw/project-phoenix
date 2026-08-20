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

const BERLIN_DATE_TIME_PARTS_FORMATTER = new Intl.DateTimeFormat("en-US", {
  timeZone: "Europe/Berlin",
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hourCycle: "h23",
});

function berlinDateFormatter(locale: string): Intl.DateTimeFormat {
  return new Intl.DateTimeFormat(locale, {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    timeZone: "Europe/Berlin",
  });
}

function chatDayMonthFormatter(locale: string): Intl.DateTimeFormat {
  return new Intl.DateTimeFormat(locale, {
    day: "2-digit",
    month: "2-digit",
    timeZone: "Europe/Berlin",
  });
}

function chatTimeFormatter(locale: string): Intl.DateTimeFormat {
  return new Intl.DateTimeFormat(locale, {
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "Europe/Berlin",
  });
}

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
 * The Berlin calendar day an instant falls on, as a Date at LOCAL midnight —
 * the shape the date pickers render and `endOfBerlinDayISO` reads back.
 *
 * Use this to seed a date picker from a cut-off written by
 * `endOfBerlinDayISO`. Loading it with a plain `new Date(iso)` shows the
 * FOLLOWING day in any browser east of Berlin (23:59 Berlin is already past
 * midnight there), so saving an otherwise untouched form would silently push
 * the date one day out.
 *
 * Returns null for an unparseable value so a malformed timestamp renders as
 * "no date" instead of throwing inside the picker.
 */
export function berlinDayFromISO(value: string): Date | null {
  const instant = new Date(value);
  if (Number.isNaN(instant.getTime())) return null;
  return parseISODate(berlinTodayISO(instant));
}

/**
 * Serializes the selected calendar day as 23:59:59 in the school's
 * Europe/Berlin timezone. The Date carries calendar fields from the date
 * picker, so those fields deliberately remain local to the person selecting
 * the day; only the resulting deadline instant is pinned to Berlin.
 */
export function endOfBerlinDayISO(date: Date): string {
  const nominalUtc = new Date(
    Date.UTC(date.getFullYear(), date.getMonth(), date.getDate(), 23, 59, 59),
  );
  const parts = BERLIN_DATE_TIME_PARTS_FORMATTER.formatToParts(nominalUtc);
  const get = (type: string) =>
    Number(parts.find((part) => part.type === type)?.value ?? "0");
  const berlinWallClockAsUtc = Date.UTC(
    get("year"),
    get("month") - 1,
    get("day"),
    get("hour"),
    get("minute"),
    get("second"),
  );
  const berlinOffsetMs = berlinWallClockAsUtc - nominalUtc.getTime();
  return new Date(nominalUtc.getTime() - berlinOffsetMs).toISOString();
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
 * Format a date string for the requested locale.
 * @param dateString ISO date string
 * @param includeWeekday Whether to include the weekday in the format
 * @returns Formatted date string (e.g., "15.12.2023" or "Freitag, 15. Dezember 2023")
 */
export function formatDate(
  dateString: string,
  includeWeekday = false,
  locale = "de-DE",
): string {
  const date = ISO_DATE_RE.test(dateString)
    ? parseISODate(dateString)
    : new Date(dateString);
  if (includeWeekday) {
    return date.toLocaleDateString(locale, {
      weekday: "long",
      year: "numeric",
      month: "long",
      day: "numeric",
    });
  }
  return date.toLocaleDateString(locale, {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

/**
 * Format an instant as the German calendar date of the school's timezone
 * ("20.07.2026"), regardless of where the viewer sits.
 *
 * Use this — not `formatDate` — for a cut-off that was written with
 * `endOfBerlinDayISO` (Antwortfrist, Ablaufdatum). Such a value is a calendar
 * day carried as 23:59:59 Berlin; rendering it in the viewer's local timezone
 * shows the FOLLOWING day east of Berlin, so a guardian in Moscow would read a
 * deadline that has in fact already passed. Plain instants (published_at,
 * created_at) keep using `formatDate`.
 *
 * Falls back to the raw input for an unparseable value, matching the chat
 * formatters — a malformed timestamp must not blank the surrounding line.
 */
export function formatBerlinDate(dateString: string, locale = "de-DE"): string {
  const date = new Date(dateString);
  if (Number.isNaN(date.getTime())) return dateString;
  return berlinDateFormatter(locale).format(date);
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
export function formatChatTime(iso: string, locale = "de-DE"): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  const dayMonth = chatDayMonthFormatter(locale).format(date);
  return `${dayMonth}, ${chatTimeFormatter(locale).format(date)}`;
}

/**
 * Full chat timestamp with year ("12.03.2026, 14:30") for the message list and
 * thread headers. Returns "" for missing input; invalid ISO falls back to the
 * raw input. Single source for the staff inbox/thread pages and the parents
 * messages list. Both portions use the school timezone so full and compact
 * chat timestamps cannot disagree for viewers outside Europe/Berlin.
 */
export function formatChatDateTime(
  iso: string | undefined,
  locale = "de-DE",
): string {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return `${berlinDateFormatter(locale).format(date)}, ${chatTimeFormatter(locale).format(date)}`;
}

/** Clock-only chat timestamp for narrow list rows. */
export function formatChatClockTime(iso: string, locale = "de-DE"): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return chatTimeFormatter(locale).format(date);
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

/**
 * Wie lange etwas schon liegt, als kurze deutsche Angabe ("heute",
 * "gestern", "vor 5 Tagen"). Für Warteschlangen, in denen das Alter eines
 * Eintrags die eigentliche Dringlichkeit trägt und ein Datum erst im Kopf
 * ausgerechnet werden müsste.
 *
 * Gezählt werden Berliner Kalendertage, nicht 24-Stunden-Blöcke: eine
 * Anfrage von gestern 23:00 Uhr ist heute früh "gestern" und nicht "heute".
 * `at` ist injizierbar, damit Tests nicht an der echten Uhr hängen.
 *
 * Gibt null zurück, wenn der Wert nicht lesbar ist — dann zeigt die Zeile
 * eben kein Alter, statt "NaN" zu schreiben.
 */
export function relativeDaysLabel(
  dateString: string,
  at: Date = new Date(),
): string | null {
  const then = berlinDayFromISO(dateString);
  if (then === null) return null;
  const today = parseISODate(berlinTodayISO(at));
  const days = Math.round(
    (today.getTime() - then.getTime()) / (24 * 60 * 60 * 1000),
  );
  if (Number.isNaN(days) || days < 0) return null;
  if (days === 0) return "heute";
  if (days === 1) return "gestern";
  return `vor ${days} Tagen`;
}
