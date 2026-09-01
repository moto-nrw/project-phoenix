// lib/meal-plan-api.ts
// API client for the meal plan (Essensplan). Staff read a week, upsert a day,
// and delete a day; the feature is gated per tenant by the backend.

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
  participants: DailyMealParticipant[];
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
    throw new Error(`Failed to fetch meal plan: ${response.statusText}`);
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
    throw new Error(`Failed to save meal plan day: ${response.statusText}`);
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
    throw new Error(
      `Failed to fetch meal participants: ${response.statusText}`,
    );
  }
  const json = (await response.json()) as {
    data?: {
      date: string;
      cutoff_time: string;
      participants: Array<{
        student_id: number | string;
        first_name: string;
        last_name: string;
        school_class: string;
      }>;
    };
  };
  const data = json.data;
  if (!data) throw new Error("Meal participant response is missing data");
  return {
    date: data.date,
    cutoffTime: data.cutoff_time,
    participants: data.participants.map((participant) => ({
      studentId: participant.student_id.toString(),
      firstName: participant.first_name,
      lastName: participant.last_name,
      schoolClass: participant.school_class,
    })),
  };
}

export async function downloadDailyMealParticipants(
  date: string,
  format: "pdf" | "xlsx",
): Promise<void> {
  const response = await sessionFetch(
    `/api/meal-plan/participants/export?date=${encodeURIComponent(date)}&format=${format}`,
    { credentials: "include" },
  );
  if (!response.ok) {
    throw new Error(
      `Failed to export meal participants: ${response.statusText}`,
    );
  }
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `mittagessen-${date}.${format}`;
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
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
