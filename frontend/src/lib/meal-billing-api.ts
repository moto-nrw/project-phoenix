// lib/meal-billing-api.ts
// Client of the Mensa-Monatsfreigabe (#4686): the OGS-Leitung checks who
// ate in a month and releases it for the Träger's billing. Calendar dates
// stay "YYYY-MM-DD"; backend int64 IDs leave as strings.

import { apiErrorFromResponse } from "./api-error";
import { sessionFetch } from "./session-cache";

export type MealBillingStatus = "running" | "open" | "released" | "expired";

export interface MealBillingSummary {
  readonly month: string;
  readonly status: MealBillingStatus;
  readonly releasedAt: string | null;
  readonly childCount: number | null;
  readonly mealCount: number | null;
}

export interface MealBillingChild {
  readonly studentId: string;
  readonly firstName: string;
  readonly lastName: string;
  readonly schoolClass: string;
  readonly registeredDays: number;
  readonly mealDays: number;
  /** Registered school days without attendance; empty once released. */
  readonly absentDates: readonly string[];
}

export interface MealBillingMonth {
  readonly month: string;
  readonly status: MealBillingStatus;
  readonly schoolDays: number;
  readonly releasedAt: string | null;
  readonly childCount: number;
  readonly mealCount: number;
  readonly absenceCount: number;
  readonly children: readonly MealBillingChild[];
  /** The computed state the release sends back; empty once released. */
  readonly fingerprint: string;
  /** The release shown, 0 before the first. */
  readonly version: number;
  /** Every release of the month, newest first. */
  readonly releases: readonly MealBillingRelease[];
  /** The month as counted now when it differs from its release (#4686). */
  readonly current: MealBillingMonth | null;
}

/** One release of a month: when, why (empty for the first) and its counts. */
interface MealBillingRelease {
  readonly version: number;
  readonly releasedAt: string;
  readonly reason: string;
  readonly childCount: number;
  readonly mealCount: number;
}

interface BackendSummary {
  month: string;
  status: MealBillingStatus;
  released_at: string | null;
  child_count: number | null;
  meal_count: number | null;
}

interface BackendMonth {
  month: string;
  status: MealBillingStatus;
  school_days: number;
  released_at: string | null;
  child_count: number;
  meal_count: number;
  absence_count: number;
  fingerprint?: string;
  version?: number;
  releases?:
    | {
        version: number;
        released_at: string;
        reason: string;
        child_count: number;
        meal_count: number;
      }[]
    | null;
  current?: BackendMonth | null;
  children:
    | {
        student_id: number;
        first_name: string;
        last_name: string;
        school_class: string;
        registered_days: number;
        meal_days: number;
        absent_dates: string[] | null;
      }[]
    | null;
}

const PATH = "/api/meal-billing-months";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await sessionFetch(path, {
    credentials: "include",
    ...init,
  });
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "meal billing request failed");
  }
  const body = (await response.json()) as { data: T };
  return body.data;
}

function monthFromBackend(body: BackendMonth): MealBillingMonth {
  return {
    month: body.month,
    status: body.status,
    schoolDays: body.school_days,
    releasedAt: body.released_at,
    childCount: body.child_count,
    mealCount: body.meal_count,
    absenceCount: body.absence_count,
    fingerprint: body.fingerprint ?? "",
    version: body.version ?? 0,
    releases: (body.releases ?? []).map((release) => ({
      version: release.version,
      releasedAt: release.released_at,
      reason: release.reason,
      childCount: release.child_count,
      mealCount: release.meal_count,
    })),
    current: body.current ? monthFromBackend(body.current) : null,
    children: (body.children ?? []).map((child) => ({
      studentId: String(child.student_id),
      firstName: child.first_name,
      lastName: child.last_name,
      schoolClass: child.school_class,
      registeredDays: child.registered_days,
      mealDays: child.meal_days,
      absentDates: child.absent_dates ?? [],
    })),
  };
}

/** The current month and the eleven before it, the latest first. */
export async function fetchMealBillingMonths(): Promise<MealBillingSummary[]> {
  const body = await request<BackendSummary[] | null>(PATH);
  return (body ?? []).map((summary) => ({
    month: summary.month,
    status: summary.status,
    releasedAt: summary.released_at,
    childCount: summary.child_count,
    mealCount: summary.meal_count,
  }));
}

export async function fetchMealBillingMonth(
  month: string,
): Promise<MealBillingMonth> {
  return monthFromBackend(
    await request<BackendMonth>(`${PATH}/${encodeURIComponent(month)}`),
  );
}

/**
 * Releases the month the OGS-Leitung checked: its children and meals. A
 * released month is released again with a reason after a correction.
 */
export async function releaseMealBillingMonth(
  month: Pick<
    MealBillingMonth,
    "month" | "childCount" | "mealCount" | "fingerprint"
  >,
  reason = "",
): Promise<MealBillingMonth> {
  return monthFromBackend(
    await request<BackendMonth>(
      `${PATH}/${encodeURIComponent(month.month)}/release`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          child_count: month.childCount,
          meal_count: month.mealCount,
          fingerprint: month.fingerprint,
          reason,
        }),
      },
    ),
  );
}
