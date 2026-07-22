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
