/**
 * Date and time utility functions for formatting and grouping
 */

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
  const date = new Date(dateString);
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

/** Time range filter options for history views */
export type TimeRangeFilter = "today" | "7days" | "week" | "month";

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
