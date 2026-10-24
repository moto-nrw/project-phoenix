// lib/meal-plan-api.ts
// API client for the meal plan (Essensplan). Staff read a week, upsert a day,
// and delete a day; the feature is gated per tenant by the backend.

import { apiErrorFromResponse } from "./api-error";
import { downloadBlob, filenameFromDisposition } from "./file-download";
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
  // RFC 3339 instant when the day's order is fixed (#3873).
  deadline: string;
  // A closing day or public holiday: nobody eats.
  closed: boolean;
  participants: DailyMealParticipant[];
  // Enrolled children who do not eat that day; only when asked for.
  others: DailyMealParticipant[];
}

export interface WeeklyMealList {
  weekStart: string;
  // closed: a closing day or public holiday, nobody eats.
  days: Array<{
    date: string;
    closed: boolean;
    participants: DailyMealParticipant[];
  }>;
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

interface BackendDailyParticipant {
  student_id: string;
  first_name: string;
  last_name: string;
  school_class: string;
}

function dailyParticipant(participant: BackendDailyParticipant) {
  return {
    studentId: participant.student_id,
    firstName: participant.first_name,
    lastName: participant.last_name,
    schoolClass: participant.school_class,
  };
}

/**
 * The day list; with `includeOthers` it also names the enrolled children who
 * do not eat that day, for the school's late changes (#3873).
 */
export async function getDailyMealParticipants(
  date: string,
  includeOthers = false,
): Promise<DailyMealList> {
  const response = await sessionFetch(
    `/api/meal-plan/participants?date=${encodeURIComponent(date)}${includeOthers ? "&include=all" : ""}`,
    { credentials: "include" },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "meal participants load failed");
  }
  const json = (await response.json()) as {
    data?: {
      date: string;
      cutoff_time: string;
      deadline: string;
      closed: boolean;
      participants: BackendDailyParticipant[];
      others?: BackendDailyParticipant[];
    };
  };
  const data = json.data;
  if (!data) throw new Error("Meal participant response is missing data");
  return {
    date: data.date,
    cutoffTime: data.cutoff_time,
    deadline: data.deadline,
    closed: data.closed === true,
    participants: data.participants.map(dailyParticipant),
    others: (data.others ?? []).map(dailyParticipant),
  };
}

/** A child's regular meal days as the school sees them (#3873). */
export interface ChildMealSchedule {
  weekdays: number[];
  effectiveFrom: string | null;
}

export async function getChildMealSchedule(
  studentId: string,
  date: string,
): Promise<ChildMealSchedule> {
  const response = await sessionFetch(
    `/api/meal-plan/participation/${encodeURIComponent(studentId)}?from=${encodeURIComponent(date)}&to=${encodeURIComponent(date)}`,
    { credentials: "include" },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "meal schedule load failed");
  }
  const json = (await response.json()) as {
    data?: { weekdays: number[]; effective_from?: string };
  };
  return {
    weekdays: json.data?.weekdays ?? [],
    effectiveFrom: json.data?.effective_from ?? null,
  };
}

/** The school registers (true) or cancels (false) a child's day. */
export async function setChildMealDay(
  studentId: string,
  date: string,
  participating: boolean,
): Promise<void> {
  const response = await sessionFetch(
    `/api/meal-plan/participation/${encodeURIComponent(studentId)}/days/${encodeURIComponent(date)}`,
    {
      method: "PUT",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ participating }),
    },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "meal day change failed");
  }
}

/** The school returns a child's day to the regular days. */
export async function clearChildMealDay(
  studentId: string,
  date: string,
): Promise<void> {
  const response = await sessionFetch(
    `/api/meal-plan/participation/${encodeURIComponent(studentId)}/days/${encodeURIComponent(date)}`,
    { method: "DELETE", credentials: "include" },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "meal day reset failed");
  }
}

/** The school sets a child's regular days from a day on. */
export async function replaceChildMealSchedule(
  studentId: string,
  weekdays: number[],
  effectiveFrom: string,
): Promise<void> {
  const response = await sessionFetch(
    `/api/meal-plan/participation/${encodeURIComponent(studentId)}/schedule`,
    {
      method: "PUT",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ weekdays, effective_from: effectiveFrom }),
    },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "meal schedule change failed");
  }
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
      days: Array<{
        date: string;
        closed?: boolean;
        participants: BackendDailyParticipant[];
      }>;
    };
  };
  const data = json.data;
  if (!data)
    throw new Error("Weekly meal participant response is missing data");
  return {
    weekStart: data.week_start,
    days: data.days.map((day) => ({
      date: day.date,
      closed: day.closed === true,
      participants: day.participants.map(dailyParticipant),
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

/** Where a day's order stands (#3873). */
type MealOrderStatus = "open" | "ordered" | "confirmed" | "unordered";

export interface MealOrderDay {
  date: string;
  closed: boolean;
  // RFC 3339 instant: the deadline of the day or of its order.
  deadline: string;
  status: MealOrderStatus;
  confirmedAt: string | null;
  total: number;
  classes: { schoolClass: string; count: number }[];
  changes: { sequence: number; schoolClass: string; delta: number }[];
}

export interface MealOrders {
  rhythm: "daily" | "weekly";
  detail: "total" | "by_class";
  days: MealOrderDay[];
}

/** The school's orders of [from, to]; reading fixes every due order. */
export async function getMealOrders(
  from: string,
  to: string,
): Promise<MealOrders> {
  const response = await sessionFetch(
    `/api/meal-plan/orders?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
    { credentials: "include" },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "meal orders load failed");
  }
  const json = (await response.json()) as {
    data?: {
      rhythm: "daily" | "weekly";
      detail: "total" | "by_class";
      days: Array<{
        date: string;
        closed: boolean;
        deadline: string;
        status: MealOrderStatus;
        confirmed_at?: string;
        total: number;
        classes: { school_class: string; count: number }[];
        changes: { sequence: number; school_class: string; delta: number }[];
      }>;
    };
  };
  const data = json.data;
  if (!data) throw new Error("Meal orders response is missing data");
  return {
    rhythm: data.rhythm,
    detail: data.detail,
    days: data.days.map((day) => ({
      date: day.date,
      closed: day.closed,
      deadline: day.deadline,
      status: day.status,
      confirmedAt: day.confirmed_at ?? null,
      total: day.total,
      classes: day.classes.map((count) => ({
        schoolClass: count.school_class,
        count: count.count,
      })),
      changes: day.changes.map((change) => ({
        sequence: change.sequence,
        schoolClass: change.school_class,
        delta: change.delta,
      })),
    })),
  };
}

/** Downloads the ordered meals per child of a month ("YYYY-MM"). */
export async function downloadMealMonth(
  month: string,
  format: "pdf" | "xlsx",
): Promise<void> {
  const response = await sessionFetch(
    `/api/meal-plan/orders/month/export?month=${encodeURIComponent(month)}&format=${format}`,
    { credentials: "include" },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "meal month export failed");
  }
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `mittagessen-${month}.${format}`;
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

/**
 * Downloads the school's delivery list of [from, to] (#3873): the same
 * document its caterer gets, without names.
 */
export async function downloadMealDeliveries(
  from: string,
  to: string,
  format: "pdf" | "xlsx" | "csv",
): Promise<void> {
  const params = new URLSearchParams({ from, to, format });
  const response = await sessionFetch(
    `/api/meal-caterer/orders/export?${params.toString()}`,
    { credentials: "include" },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "meal delivery export failed");
  }
  downloadBlob(
    await response.blob(),
    filenameFromDisposition(response) ??
      `lieferliste-${from}-bis-${to}.${format}`,
  );
}
