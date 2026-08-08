// Client-side API for the Lehrkraft-Tagesansicht (#1772): read-only per-class
// day view. The backend scopes every call to the caller's assigned classes
// (education.class_teachers) and never returns guardian contact data here.

import { authFetch } from "./api-helpers";

export interface ClassDayRow {
  student_id: number;
  first_name: string;
  last_name: string;
  group_name?: string;
  registered: boolean;
  stays_today: boolean;
  offerings: string[];
  arrival?: string;
  pickup?: string;
  departure?: string;
  status?: "sick" | "excused" | "class_trip" | "cancelled" | "";
}

export interface ClassDayTotals {
  students: number;
  staying: number;
  leaving: number;
  absent: number;
}

export interface ClassDayReport {
  school_class: string;
  date: string; // YYYY-MM-DD
  weekday: string; // "mon".."fri", "" am Wochenende
  school_day: boolean;
  phase_name?: string;
  totals: ClassDayTotals;
  rows: ClassDayRow[];
}

interface ProxyResponse<T> {
  success: boolean;
  data: T;
}

export async function fetchMyClasses(): Promise<string[]> {
  const response = await authFetch<ProxyResponse<{ classes: string[] }>>(
    "/api/class-day/classes",
  );
  return response.data.classes ?? [];
}

export async function fetchClassDay(
  schoolClass: string,
  date: string,
): Promise<ClassDayReport> {
  const params = new URLSearchParams({ class: schoolClass, date });
  const response = await authFetch<ProxyResponse<ClassDayReport>>(
    `/api/class-day?${params.toString()}`,
  );
  return response.data;
}
