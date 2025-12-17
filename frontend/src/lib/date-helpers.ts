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
      const date = new Date(timestamp).toLocaleDateString("de-DE");
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
export function formatDate(
  dateString: string,
  includeWeekday = false,
): string {
  const date = new Date(dateString);
  if (includeWeekday) {
    return date.toLocaleDateString("de-DE", {
      weekday: "long",
      year: "numeric",
      month: "long",
      day: "numeric",
    });
  }
  return date.toLocaleDateString("de-DE");
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
    return `${hours} Std. ${mins > 0 ? `${mins} Min.` : ""}`.trim();
  } else {
    return `${mins} Min.`;
  }
}
