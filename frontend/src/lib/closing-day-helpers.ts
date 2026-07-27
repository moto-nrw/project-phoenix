// OGS-Schließtage (#1418 3b) — domain types and mappers.
//
// Closing days are tenant-defined closure ranges (pädagogische Tage,
// Weihnachtswoche, Sommerschließung) on top of the computed public holidays.
// On these days the backend resolves the staff Soll to 0, exactly like on
// holidays.
//
// Backend reference: backend/models/schedule/closing_day.go,
// backend/api/timetable/closing_days.go (ClosingDayRequest / Response).

import { formatDate } from "~/lib/date-helpers";

/** Backend wire shape (snake_case, int64 IDs). */
export interface BackendClosingDay {
  id: number;
  start_date: string; // YYYY-MM-DD
  end_date: string; // YYYY-MM-DD
  reason: string;
  created_at: string;
  updated_at: string;
}

export interface ClosingDay {
  id: string;
  startDate: string; // YYYY-MM-DD
  endDate: string; // YYYY-MM-DD
  reason: string;
}

/** Create/update payload — matches ClosingDayRequest on the backend. */
export interface ClosingDayInput {
  start_date: string;
  end_date: string;
  reason: string;
}

export function mapClosingDay(data: BackendClosingDay): ClosingDay {
  return {
    id: data.id.toString(),
    startDate: data.start_date.slice(0, 10),
    endDate: data.end_date.slice(0, 10),
    reason: data.reason,
  };
}

export function mapClosingDays(data: BackendClosingDay[]): ClosingDay[] {
  return data.map(mapClosingDay);
}

/** "24.12.2026 – 31.12.2026", collapsed to one date for single-day ranges. */
export function formatClosingDayRange(day: ClosingDay): string {
  if (day.startDate === day.endDate) {
    return formatDate(day.startDate);
  }
  return `${formatDate(day.startDate)} – ${formatDate(day.endDate)}`;
}

/** Ein Schließtag-Zeitraum ohne Identität — was die Tages-Expansion braucht. */
export type ClosingDayRange = Omit<ClosingDay, "id">;

/**
 * Grund des Schließtags an einem einzelnen Kalendertag, oder `undefined`.
 * Für Datumsfelder, deren Wert außerhalb des sichtbaren Fensters liegen kann
 * (Termin-Dialog, „Verschieben nach“). Der erste passende Zeitraum gewinnt.
 */
export function findClosingDayReason(
  ranges: readonly ClosingDayRange[] | null | undefined,
  dateISO: string,
): string | undefined {
  if (dateISO === "") return undefined;
  const day = dateISO.slice(0, 10);
  for (const entry of ranges ?? []) {
    if (
      entry.startDate.slice(0, 10) <= day &&
      day <= entry.endDate.slice(0, 10)
    ) {
      return entry.reason;
    }
  }
  return undefined;
}

export interface ClosingDayConflict {
  dateISO: string;
  reason: string;
}

/** First generated date that overlaps a stored closing-day range. */
export function findFirstClosingDayConflict(
  ranges: readonly ClosingDayRange[] | null | undefined,
  dates: readonly string[],
): ClosingDayConflict | null {
  for (const dateISO of dates) {
    const reason = findClosingDayReason(ranges, dateISO);
    if (reason !== undefined) return { dateISO, reason };
  }
  return null;
}

/**
 * Expands stored closure ranges into a per-day lookup (YYYY-MM-DD → Grund),
 * clamped to [from, to]. Shared by every surface that marks closing days:
 * the Zeiterfassung tables (#1418 3b) and the planning grids (#2032). The
 * first range wins on overlap, matching the order the backend returns.
 */
export function expandClosingDaysToMap(
  ranges: readonly ClosingDayRange[] | null | undefined,
  from: string,
  to: string,
): ReadonlyMap<string, string> {
  const closingDays = new Map<string, string>();
  const windowStart = from.slice(0, 10);
  const windowEnd = to.slice(0, 10);
  if (windowEnd < windowStart) return closingDays;

  for (const entry of ranges ?? []) {
    const storedStart = entry.startDate.slice(0, 10);
    const storedEnd = entry.endDate.slice(0, 10);
    const start = storedStart < windowStart ? windowStart : storedStart;
    const end = storedEnd > windowEnd ? windowEnd : storedEnd;
    if (end < start) continue;

    const startDate = new Date(`${start}T00:00:00`);
    const endDate = new Date(`${end}T00:00:00`);
    // Math.round (not floor) keeps the count DST-safe across 23/25-hour days;
    // setDate walks local calendar days. Do not cap this window: calendar
    // periods may validly exceed the time-tracking endpoint's 400-day request
    // limit, and planning views must still mark every configured closure.
    const dayCount = Math.max(
      0,
      Math.round((endDate.getTime() - startDate.getTime()) / 86_400_000) + 1,
    );
    for (let offset = 0; offset < dayCount; offset++) {
      const cursor = new Date(startDate);
      cursor.setDate(cursor.getDate() + offset);
      const key = `${cursor.getFullYear()}-${String(cursor.getMonth() + 1).padStart(2, "0")}-${String(cursor.getDate()).padStart(2, "0")}`;
      if (!closingDays.has(key)) {
        closingDays.set(key, entry.reason);
      }
    }
  }
  return closingDays;
}
