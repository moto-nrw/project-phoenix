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

export interface EnrollablePhase {
  readonly school_id: string;
  readonly school_name: string;
  readonly school_slug: string;
  readonly phase_id: string;
  readonly phase_name: string;
  readonly phase_kind: string;
  readonly service_start_date: string; // ISO date
  readonly service_end_date: string; // ISO date
  readonly enrollment_open_at?: string; // ISO timestamp
  readonly enrollment_close_at?: string; // ISO timestamp
  readonly already_linked: boolean;
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

interface ApiEnvelope<T> {
  readonly status?: string;
  readonly data?: T;
}

async function getJson<T>(url: string): Promise<T> {
  const response = await fetch(url, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });

  if (!response.ok) {
    let message = `Request failed (${response.status})`;
    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // Body was not JSON, keep the generic message.
    }
    const context = { url, status: response.status, message };
    if (response.status === 401) {
      logger.warn("parent_api_request_failed", context);
      if (typeof window !== "undefined") {
        window.location.assign("/parents/login");
      }
    } else {
      logger.error("parent_api_request_failed", context);
    }
    throw new Error(message);
  }

  const json = (await response.json()) as ApiEnvelope<T>;
  // Backend wraps in { status, data }; some routes return data directly.
  if (json && typeof json === "object" && "data" in json) {
    return json.data as T;
  }
  return json as unknown as T;
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
 * Fetches every (school, open phase) pair the parent could enroll a
 * new child at. The backend sorts schools the parent is already linked
 * to first, then by school name. Used by the school picker on the
 * Neue Anmeldung flow.
 */
export async function listEnrollableSchools(): Promise<EnrollablePhase[]> {
  return getJson<EnrollablePhase[]>("/api/parent/me/enrollable-schools");
}

/**
 * Fetches every enrollment.requests row owned by the calling parent's
 * account, joined to phase + school + child summaries. Newest first.
 * Powers the "Anmeldungen in Bearbeitung" section on the dashboard.
 */
export async function listMyEnrollments(): Promise<EnrollmentRequest[]> {
  return getJson<EnrollmentRequest[]>("/api/parent/me/enrollments");
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
