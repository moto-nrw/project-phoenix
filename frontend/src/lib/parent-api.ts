/**
 * Parent-portal client API. Symmetric to the operator-api / tenant
 * api-helpers split. Every call goes through a Next.js proxy route
 * under /api/parent/* which forwards (with the parent NextAuth
 * session token) to the backend /parent/* endpoints.
 *
 * Types here mirror backend/api/parent/me_handlers.go. int64 ids
 * arrive as strings per the project's frontend convention.
 */

import { createLogger } from "~/lib/logger";
import type { AppLocale } from "~/i18n/locales";
import type {
  MeProfileResponse,
  SubmitEnrollmentPayload,
  SubmitEnrollmentResult,
} from "~/lib/enrollment-submission-api";

const logger = createLogger({ component: "ParentAPI" });

export type ChildStatus = "pending" | "active" | "inactive" | "alumnus";

export interface Child {
  readonly student_id: string;
  readonly tenant_id: string;
  readonly first_name: string;
  readonly last_name: string;
  readonly school_class?: string;
  readonly status: ChildStatus;
  readonly enrolled_from?: string; // ISO date
  readonly enrolled_until?: string; // ISO date
  readonly school_name: string;
  readonly school_slug: string;
}

// Per-child status values exposed on the enrollment-requests list.
// Mirrors models/enrollment ChildStatus* constants.
export type EnrollmentChildStatus =
  | "submitted"
  | "under_review"
  | "approved"
  | "waitlisted"
  | "rejected"
  | "withdrawn";

interface EnrollmentRequestChild {
  readonly child_id: string;
  readonly first_name: string;
  readonly last_name: string;
  readonly status: EnrollmentChildStatus;
  readonly status_reason?: string;
}

export interface EnrollmentRequest {
  readonly request_id: string;
  readonly tenant_id: string;
  readonly status_token: string;
  readonly submitted_at: string; // ISO timestamp
  readonly withdrawn_at?: string; // ISO timestamp
  readonly phase_id: string;
  readonly phase_name: string;
  readonly service_start_date: string; // ISO date
  readonly service_end_date: string; // ISO date
  readonly school_name: string;
  readonly school_slug: string;
  readonly children: EnrollmentRequestChild[];
}

export interface ParentProfile {
  // null = the guardian has never picked a parents-portal language, so the
  // client keeps the anonymous cookie/Accept-Language locale.
  readonly portal_locale: AppLocale | null;
}

type StudentStatusKind = "sick" | "excused";

// One reported sick/excused day. Mirrors api/parent.StatusDayResponse.
export interface StatusDay {
  readonly id: string;
  readonly student_id: string;
  readonly date: string; // YYYY-MM-DD
  readonly status: StudentStatusKind;
  readonly reported_at: string; // ISO timestamp
  readonly source: string;
  readonly note?: string;
}

// One parent note. Mirrors api/parent.ParentNoteResponse, newest-first.
export interface ParentNote {
  readonly id: string;
  readonly student_id: string;
  readonly body: string;
  readonly created_at: string; // ISO timestamp
}

// Resolved per-tenant parent-portal feature toggles for a child.
export interface ChildFeatures {
  readonly sick_note_enabled: boolean;
  readonly notes_enabled: boolean;
  readonly pickup_change_enabled: boolean;
}

// One day's pickup/arrival override. Mirrors api/parent.CareExceptionResponse.
// Times are "HH:MM" wall-clock strings; a missing leg has no override that day.
// `source` is "guardian" for parent-set entries, "staff" for ones the team set.
export interface CareException {
  readonly date: string;
  readonly pickup_time?: string;
  readonly arrival_time?: string;
  readonly source: string;
  readonly updated_at: string;
}

interface ApiEnvelope<T> {
  readonly status?: string;
  readonly data?: T;
}

/**
 * Unwraps the backend's `{ status, data }` envelope. Some routes return the
 * payload directly, so a response without a `data` key is passed through as-is.
 * Shared by every GET/PUT helper here so envelope handling lives in one place.
 */
function unwrapEnvelope<T>(json: ApiEnvelope<T>): T {
  if (json && typeof json === "object" && "data" in json) {
    return json.data as T;
  }
  return json as unknown as T;
}

/**
 * A failed parents-portal API call. Carries the HTTP status and the backend's
 * stable error `code` (e.g. "care_exception_conflict") so callers can map to a
 * localized message instead of showing the raw English error string. Extends
 * `Error`, so existing `err instanceof Error ? err.message` handling still works.
 */
export class ParentApiError extends Error {
  readonly status: number;
  readonly code?: string;
  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = "ParentApiError";
    this.status = status;
    this.code = code;
  }
}

/**
 * Reads the backend error message + code off a failed response, logs it, and
 * (on 401) bounces the user to the parents login. Shared by
 * getJson/postJson/deleteJson so the error/redirect/logging path lives in
 * exactly one place. Always throws a ParentApiError.
 */
async function throwResponseError(
  url: string,
  response: Response,
): Promise<never> {
  let message = `Request failed (${response.status})`;
  let code: string | undefined;
  try {
    const body = (await response.json()) as { error?: string; code?: string };
    if (body.error) message = body.error;
    if (body.code) code = body.code;
  } catch {
    // Body was not JSON, keep the generic message.
  }
  const context = { url, status: response.status, message, code };
  if (response.status === 401) {
    logger.warn("parent_api_request_failed", context);
    if (typeof window !== "undefined") {
      window.location.assign("/parents/login");
    }
  } else {
    logger.error("parent_api_request_failed", context);
  }
  throw new ParentApiError(message, response.status, code);
}

async function getJson<T>(url: string): Promise<T> {
  const response = await fetch(url, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
  if (!response.ok) await throwResponseError(url, response);
  return unwrapEnvelope((await response.json()) as ApiEnvelope<T>);
}

async function postJson<T>(url: string, body: unknown): Promise<T> {
  const response = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!response.ok) await throwResponseError(url, response);
  return unwrapEnvelope((await response.json()) as ApiEnvelope<T>);
}

async function deleteJson<T>(url: string): Promise<T> {
  const response = await fetch(url, {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
  });
  if (!response.ok) await throwResponseError(url, response);
  return unwrapEnvelope((await response.json()) as ApiEnvelope<T>);
}

/**
 * Fetches every child linked to the calling parent's account, across
 * every active tenant mapping. The response is already sorted (school
 * to first name to last name) by the backend.
 */
export async function listMyChildren(): Promise<Child[]> {
  return getJson<Child[]>("/api/parent/me/children");
}

/**
 * Fetches every enrollment.requests row owned by the calling parent's
 * account, joined to phase + school + child summaries. Newest first.
 * Powers the "Anmeldungen in Bearbeitung" section on the dashboard.
 */
export async function listMyEnrollments(): Promise<EnrollmentRequest[]> {
  return getJson<EnrollmentRequest[]>("/api/parent/me/enrollments");
}

export async function fetchParentProfile(): Promise<ParentProfile> {
  return getJson<ParentProfile>("/api/parent/me/profile");
}

export async function updateParentPortalLocale(
  locale: AppLocale,
): Promise<ParentProfile> {
  const response = await fetch("/api/parent/me/profile", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ portal_locale: locale }),
  });
  if (!response.ok) {
    throw new Error(`Profile update failed (${response.status})`);
  }
  const json = (await response.json()) as ApiEnvelope<ParentProfile>;
  return unwrapEnvelope(json);
}

/**
 * Fetches the parent's autofill payload for the embedded enrollment
 * form, scoped to a specific tenant slug. Returns null on 401 so the
 * form can render without prefill rather than failing. This matches the
 * public path's fetchMyEnrollmentProfile contract.
 */
export async function fetchParentEnrollmentProfile(
  tenantSlug: string,
): Promise<MeProfileResponse | null> {
  const response = await fetch(
    `/api/parent/enrollments/${encodeURIComponent(tenantSlug)}/profile`,
    { cache: "no-store" },
  );
  if (response.status === 401) {
    return null;
  }
  if (!response.ok) {
    let message = `Profile request failed (${response.status})`;
    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // Body was not JSON, keep the generic message.
    }
    logger.error("parent_profile_request_failed", {
      tenant_slug: tenantSlug,
      status: response.status,
      message,
    });
    throw new Error(message);
  }
  const json = (await response.json()) as { data?: MeProfileResponse };
  return (json.data ?? (json as unknown as MeProfileResponse)) || null;
}

/**
 * Submits an enrollment from the parents portal. Backend stamps
 * guardian_account_id from the parent JWT and skips captcha
 * verification. The JWT itself is the anti-bot signal. Reuses the
 * same payload + result shape as the public submitEnrollment so the
 * EnrollmentForm can consume both paths interchangeably.
 */
export async function submitParentEnrollment(
  tenantSlug: string,
  payload: SubmitEnrollmentPayload,
): Promise<SubmitEnrollmentResult> {
  const response = await fetch(
    `/api/parent/enrollments/${encodeURIComponent(tenantSlug)}/submit`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    },
  );
  if (!response.ok) {
    let message = "Anmeldung konnte nicht übermittelt werden";
    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // Body was not JSON, keep the generic message.
    }
    logger.error("parent_submit_failed", {
      tenant_slug: tenantSlug,
      status: response.status,
      message,
    });
    throw new Error(message);
  }
  const json = (await response.json()) as {
    data?: SubmitEnrollmentResult;
    request_id?: string;
    status_url?: string;
  };
  if (json.data) return json.data;
  return {
    request_id: json.request_id ?? "",
    status_url: json.status_url ?? "",
  };
}

/**
 * Reports the child sick for one or more dates (YYYY-MM-DD). Returns the
 * active sick days in the submitted range. The backend verifies the
 * caller is a guardian of the child and that the school has the feature
 * enabled; a disabled school surfaces as a thrown error.
 */
export async function submitSickNote(
  studentId: string,
  dates: string[],
  reason?: string,
): Promise<StatusDay[]> {
  return postJson<StatusDay[]>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/sick-note`,
    { dates, reason: reason ?? "" },
  );
}

/**
 * Fetches which write actions the child's school allows (resolved per
 * tenant). Used to hide/disable actions the backend would reject. Defaults
 * to both enabled if the request fails, so a transient error doesn't lock a
 * parent out of an enabled feature.
 */
export async function getChildFeatures(
  studentId: string,
): Promise<ChildFeatures> {
  return getJson<ChildFeatures>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/features`,
  );
}

/**
 * Fetches the child's active sick days (today .. +2 months by default).
 * Used to show already-reported days on the child page.
 */
export async function listSickDays(studentId: string): Promise<StatusDay[]> {
  return getJson<StatusDay[]>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/sick-note`,
  );
}

/** Fetches the newest notes the parent left for the team (newest first). */
export async function listChildNotes(studentId: string): Promise<ParentNote[]> {
  return getJson<ParentNote[]>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/notes`,
  );
}

/** Appends a note and returns the newest few (newest first). */
export async function addChildNote(
  studentId: string,
  body: string,
): Promise<ParentNote[]> {
  return postJson<ParentNote[]>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/notes`,
    { body },
  );
}

/**
 * Sets a one-day pickup and/or arrival override for the child. The two times
 * are the COMPLETE override for the day: a "HH:MM" string sets that leg, an
 * omitted leg (sent as null) clears it. At least one must be present — clearing
 * the whole day goes through deleteCareException. Returns the merged override
 * for the day. The backend verifies guardianship, the feature gate, and refuses
 * to overwrite a staff-set exception (surfaced as a thrown error).
 */
export async function submitCareException(
  studentId: string,
  params: { date: string; pickupTime?: string; arrivalTime?: string },
): Promise<CareException> {
  return postJson<CareException>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/care-exception`,
    {
      date: params.date,
      pickup_time: params.pickupTime ?? null,
      arrival_time: params.arrivalTime ?? null,
    },
  );
}

/**
 * Fetches the child's pickup/arrival overrides (today .. +2 months by default),
 * staff- and parent-set alike, so the modal can show what is already in place.
 */
export async function listCareExceptions(
  studentId: string,
): Promise<CareException[]> {
  return getJson<CareException[]>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/care-exception`,
  );
}

/** Removes the parent-set override for a single date (YYYY-MM-DD). */
export async function deleteCareException(
  studentId: string,
  date: string,
): Promise<void> {
  await deleteJson<null>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/care-exception?date=${encodeURIComponent(date)}`,
  );
}
