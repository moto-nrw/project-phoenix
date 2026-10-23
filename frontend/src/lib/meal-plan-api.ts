// lib/meal-plan-api.ts
// API client for the meal plan (Essensplan). Staff read a week, upsert a day,
// and delete a day; the feature is gated per tenant by the backend.

import { apiErrorFromResponse } from "./api-error";
import { sessionFetch } from "./session-cache";
import { parseISODate, toISODate } from "./date-helpers";
import { createLogger } from "~/lib/logger";
import { downloadBlob } from "~/lib/file-download";

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

/** Die Fristen der Schule für das Mittagessen. */
export interface MealCutoffs {
  cutoffTime: string;
  /** Werktage vor dem Essenstag, an denen die Frist gilt; 0 = am selben Tag. */
  cutoffLeadDays: number;
  /**
   * 1-5 (Mo-Fr): Die Frist der ganzen Woche endet an diesem Wochentag der
   * Vorwoche; 0 = Vorlauf in Werktagen.
   */
  cutoffWeekday: number;
  /** Spätere Abmeldefrist; leer, wenn sie der Änderungsfrist entspricht. */
  cancellationCutoffTime: string;
  cancellationCutoffLeadDays: number;
}

export interface DailyMealList extends MealCutoffs {
  date: string;
  /** Feiertag oder Schließtag: kein Essen. */
  closed: boolean;
  participants: DailyMealParticipant[];
  /** Kinder des Tages ohne Anmeldung; das Team kann sie anmelden. */
  available: DailyMealParticipant[];
}

export interface WeeklyMealList extends MealCutoffs {
  weekStart: string;
  days: Array<{
    date: string;
    closed: boolean;
    participants: DailyMealParticipant[];
  }>;
}

/** Wire shape of the cutoffs (snake_case). */
export interface BackendMealCutoffs {
  cutoff_time: string;
  cutoff_lead_days?: number;
  cutoff_weekday?: number;
  cancellation_cutoff_time?: string;
  cancellation_cutoff_lead_days?: number;
}

export function mapMealCutoffs(data: BackendMealCutoffs): MealCutoffs {
  return {
    cutoffTime: data.cutoff_time,
    cutoffLeadDays: data.cutoff_lead_days ?? 0,
    cutoffWeekday: data.cutoff_weekday ?? 0,
    cancellationCutoffTime: data.cancellation_cutoff_time ?? "",
    cancellationCutoffLeadDays: data.cancellation_cutoff_lead_days ?? 0,
  };
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
    data?: BackendMealCutoffs & {
      date: string;
      closed?: boolean;
      participants: BackendMealParticipant[];
      available?: BackendMealParticipant[];
    };
  };
  const data = json.data;
  if (!data) throw new Error("Meal participant response is missing data");
  return {
    ...mapMealCutoffs(data),
    date: data.date,
    closed: data.closed ?? false,
    participants: data.participants.map(mapParticipant),
    available: (data.available ?? []).map(mapParticipant),
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
    data?: BackendMealCutoffs & {
      week_start: string;
      days: Array<{
        date: string;
        closed?: boolean;
        participants: BackendMealParticipant[];
      }>;
    };
  };
  const data = json.data;
  if (!data)
    throw new Error("Weekly meal participant response is missing data");
  return {
    ...mapMealCutoffs(data),
    weekStart: data.week_start,
    days: data.days.map((day) => ({
      date: day.date,
      closed: day.closed ?? false,
      participants: day.participants.map(mapParticipant),
    })),
  };
}

/**
 * Das Team meldet ein Kind für einen Tag an (true) oder ab (false), auch
 * nach der Frist; null setzt den Tag auf die festen Tage des Kindes zurück.
 */
export async function setMealParticipantDay(
  studentId: string,
  date: string,
  participating: boolean | null,
): Promise<void> {
  const path = `/api/meal-plan/participants/${encodeURIComponent(studentId)}/${encodeURIComponent(date)}`;
  const response = await sessionFetch(
    path,
    participating === null
      ? { method: "DELETE", credentials: "include" }
      : {
          method: "PUT",
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ participating }),
        },
  );
  if (!response.ok) {
    logger.error("meal_participant_day_change_failed", {
      status: response.status,
    });
    throw await apiErrorFromResponse(
      response,
      "meal participant day change failed",
    );
  }
}

async function downloadMealList(path: string, filename: string): Promise<void> {
  const response = await sessionFetch(path, { credentials: "include" });
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      "meal participants export failed",
    );
  }
  downloadBlob(await response.blob(), filename);
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
