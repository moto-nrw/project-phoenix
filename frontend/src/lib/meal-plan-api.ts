// lib/meal-plan-api.ts
// API client for the meal plan (Essensplan). Staff read a week, upsert a day,
// and delete a day; the feature is gated per tenant by the backend.

import { apiErrorFromResponse } from "./api-error";
import { sessionFetch } from "./session-cache";
import { parseISODate, toISODate } from "./date-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "MealPlanAPI" });

/** Wire shape returned by the backend (snake_case, date as YYYY-MM-DD). */
export interface BackendMealPlanEntry {
  date: string;
  position: number;
  dish: string;
  note?: string | null;
}

/** One dish on a day of the meal plan as used by the UI. */
export interface MealPlanEntry {
  date: string; // YYYY-MM-DD
  position: number;
  dish: string;
  note: string | null;
}

/** One dish in a day submission. */
export interface DishInput {
  dish: string;
  note: string | null;
}

export interface DailyMealParticipant {
  studentId: string;
  firstName: string;
  lastName: string;
  schoolClass: string;
}

export interface DailyMealList {
  date: string;
  cutoffTime: string;
  /** Werktage vor dem Essenstag, an denen die Frist gilt; 0 = am selben Tag. */
  cutoffLeadDays: number;
  participants: DailyMealParticipant[];
}

export interface WeeklyMealList {
  weekStart: string;
  cutoffTime: string;
  cutoffLeadDays: number;
  days: Array<{ date: string; participants: DailyMealParticipant[] }>;
}

interface BackendMealParticipant {
  student_id: string;
  first_name: string;
  last_name: string;
  school_class: string;
}

function mapParticipant(
  participant: BackendMealParticipant,
): DailyMealParticipant {
  return {
    studentId: participant.student_id,
    firstName: participant.first_name,
    lastName: participant.last_name,
    schoolClass: participant.school_class,
  };
}

function mapEntry(entry: BackendMealPlanEntry): MealPlanEntry {
  return {
    date: entry.date,
    position: entry.position,
    dish: entry.dish,
    note: entry.note?.trim() ? entry.note : null,
  };
}

function unwrap(data: { data: unknown } | unknown[]): BackendMealPlanEntry[] {
  if (Array.isArray(data)) return data as BackendMealPlanEntry[];
  if (data && typeof data === "object" && "data" in data) {
    return (data.data as BackendMealPlanEntry[]) ?? [];
  }
  return [];
}

/** Fetch the Monday-Friday entries for the week containing weekStart (ISO). */
export async function getMealPlanWeek(
  weekStart: string,
): Promise<MealPlanEntry[]> {
  const response = await sessionFetch(
    `/api/meal-plan?week_start=${encodeURIComponent(weekStart)}`,
    { credentials: "include" },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "meal plan load failed");
  }
  const data = (await response.json()) as { data: unknown } | unknown[];
  return unwrap(data).map(mapEntry);
}

/** Replace all dishes for one day (empty list clears the day). */
export async function setDay(date: string, dishes: DishInput[]): Promise<void> {
  const response = await sessionFetch(
    `/api/meal-plan/${encodeURIComponent(date)}`,
    {
      method: "PUT",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ dishes }),
    },
  );
  if (!response.ok) {
    logger.error("meal_set_day_failed", { status: response.status });
    throw await apiErrorFromResponse(response, "meal plan day save failed");
  }
}

export async function getDailyMealParticipants(
  date: string,
): Promise<DailyMealList> {
  const response = await sessionFetch(
    `/api/meal-plan/participants?date=${encodeURIComponent(date)}`,
    { credentials: "include" },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "meal participants load failed");
  }
  const json = (await response.json()) as {
    data?: {
      date: string;
      cutoff_time: string;
      cutoff_lead_days?: number;
      participants: BackendMealParticipant[];
    };
  };
  const data = json.data;
  if (!data) throw new Error("Meal participant response is missing data");
  return {
    date: data.date,
    cutoffTime: data.cutoff_time,
    cutoffLeadDays: data.cutoff_lead_days ?? 0,
    participants: data.participants.map(mapParticipant),
  };
}

/** Kitchen list of every school day in the week containing weekStart. */
export async function getWeeklyMealParticipants(
  weekStart: string,
): Promise<WeeklyMealList> {
  const response = await sessionFetch(
    `/api/meal-plan/participants/week?week_start=${encodeURIComponent(weekStart)}`,
    { credentials: "include" },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      "weekly meal participants load failed",
    );
  }
  const json = (await response.json()) as {
    data?: {
      week_start: string;
      cutoff_time: string;
      cutoff_lead_days: number;
      days: Array<{ date: string; participants: BackendMealParticipant[] }>;
    };
  };
  const data = json.data;
  if (!data)
    throw new Error("Weekly meal participant response is missing data");
  return {
    weekStart: data.week_start,
    cutoffTime: data.cutoff_time,
    cutoffLeadDays: data.cutoff_lead_days,
    days: data.days.map((day) => ({
      date: day.date,
      participants: day.participants.map(mapParticipant),
    })),
  };
}

async function downloadMealList(path: string, filename: string): Promise<void> {
  const response = await sessionFetch(path, { credentials: "include" });
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      "meal participants export failed",
    );
  }
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export async function downloadDailyMealParticipants(
  date: string,
  format: "pdf" | "xlsx",
): Promise<void> {
  await downloadMealList(
    `/api/meal-plan/participants/export?date=${encodeURIComponent(date)}&format=${format}`,
    `mittagessen-${date}.${format}`,
  );
}

export async function downloadWeeklyMealParticipants(
  weekStart: string,
  format: "pdf" | "xlsx",
): Promise<void> {
  await downloadMealList(
    `/api/meal-plan/participants/week/export?week_start=${encodeURIComponent(weekStart)}&format=${format}`,
    `mittagessen-woche-${weekStart}.${format}`,
  );
}

/** Monday (ISO) of the week containing the given Date. */
export function mondayOf(date: Date): string {
  const offset = (date.getDay() + 6) % 7; // Mon=0 .. Sun=6
  const monday = new Date(date);
  monday.setDate(date.getDate() - offset);
  return toISODate(monday);
}

/** The five working-day ISO dates (Mon-Fri) starting at mondayISO. */
export function workWeekDates(mondayISO: string): string[] {
  const monday = parseISODate(mondayISO);
  return Array.from({ length: 5 }, (_, i) => {
    const d = new Date(monday);
    d.setDate(monday.getDate() + i);
    return toISODate(d);
  });
}
