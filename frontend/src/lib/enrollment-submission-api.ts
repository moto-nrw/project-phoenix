import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentSubmissionAPI" });

export interface PublicCareOffering {
  id: string;
  phase_id?: string | null;
  name: string;
  description?: string | null;
  days_of_week_mode: "fixed" | "parent_choice";
  available_days: string[];
  includes_holiday_care: boolean;
  includes_lunch: boolean;
  capacity?: number | null;
  price_cents?: number | null;
  is_active: boolean;
}

export interface SubmitChildPayload {
  first_name: string;
  last_name: string;
  date_of_birth: string; // YYYY-MM-DD
  target_grade_level?: number;
  custom_data?: Record<string, unknown>;
  offering_ids?: number[];
}

export interface SubmitEnrollmentPayload {
  phase_id: number;
  guardian_first_name: string;
  guardian_last_name: string;
  guardian_email: string;
  guardian_phone?: string;
  consent_flags?: Record<string, unknown>;
  custom_data?: Record<string, unknown>;
  children: SubmitChildPayload[];
  captcha_token?: string;
}

export interface SubmitEnrollmentResult {
  request_id: string;
  status_url: string;
}

interface BackendEnvelope<T> {
  status?: string;
  data?: T;
  error?: string;
  message?: string;
}

async function readJSON<T>(response: Response): Promise<T> {
  const raw = (await response.json()) as BackendEnvelope<T>;
  if (
    raw &&
    typeof raw === "object" &&
    "data" in raw &&
    raw.data !== undefined
  ) {
    return raw.data as T;
  }
  return raw as unknown as T;
}

async function readError(response: Response, fallback: string): Promise<Error> {
  let message = fallback;
  try {
    const payload = (await response.json()) as BackendEnvelope<unknown>;
    message =
      payload.error ??
      payload.message ??
      `${fallback} (HTTP ${response.status})`;
  } catch {
    /* ignore */
  }
  logger.error("enrollment_submission_api_failed", {
    status: response.status,
    message,
  });
  const err = new Error(message) as Error & { status?: number };
  err.status = response.status;
  return err;
}

/**
 * Fetches the public care-offering catalog for a given tenant slug
 * and phase. Returns only is_active=true rows; the phase-level window
 * is enforced by the surrounding parent flow (PR C).
 */
export async function fetchPublicCareOfferings(
  tenantSlug: string,
  phaseId: string,
): Promise<PublicCareOffering[]> {
  const response = await fetch(
    `/api/enrollment/care-offerings/public/${encodeURIComponent(
      tenantSlug,
    )}/${encodeURIComponent(phaseId)}`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Betreuungsangebote konnten nicht geladen werden",
    );
  }
  const list = await readJSON<PublicCareOffering[]>(response);
  return Array.isArray(list) ? list : [];
}

export interface PublicPhase {
  id: string;
  name: string;
  kind: "school_year" | "holiday" | "custom";
  service_start_date: string;
  service_end_date: string;
  enrollment_open_at?: string;
  enrollment_close_at?: string;
  show_status_reason_to_parent: boolean;
}

/**
 * Fetches the currently-open phases for a tenant. Public route — no
 * JWT required. The parent landing page uses this for the phase
 * picker; clicking a phase navigates the parent into the form.
 */
export async function fetchPublicPhases(
  tenantSlug: string,
): Promise<PublicPhase[]> {
  const response = await fetch(
    `/api/enrollment/phases/public/${encodeURIComponent(tenantSlug)}`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw await readError(response, "Phasen konnten nicht geladen werden");
  }
  const list = await readJSON<PublicPhase[]>(response);
  return Array.isArray(list) ? list : [];
}

export interface MeProfileChild {
  id: string;
  first_name: string;
  last_name: string;
  school_class: string;
  grade_level?: number;
}

export interface MeProfileResponse {
  guardian: {
    first_name: string;
    last_name: string;
    email: string;
    phone?: string;
  };
  children: MeProfileChild[];
}

/**
 * Best-effort autofill payload for parents who already have a tenant
 * session. Returns null when not authenticated (HTTP 401) so the
 * caller can render the public form unchanged. Other errors propagate
 * as Error so SWR/effects can surface them.
 */
export async function fetchMyEnrollmentProfile(): Promise<MeProfileResponse | null> {
  const response = await fetch("/api/enrollment/me/profile", {
    cache: "no-store",
  });
  if (response.status === 401) {
    return null;
  }
  if (!response.ok) {
    throw await readError(response, "Profil konnte nicht geladen werden");
  }
  return readJSON<MeProfileResponse>(response);
}

/**
 * Submits an enrollment for the given tenant slug. The backend handles
 * captcha verification, schema pinning, and outbox enqueueing.
 */
export async function submitEnrollment(
  tenantSlug: string,
  payload: SubmitEnrollmentPayload,
): Promise<SubmitEnrollmentResult> {
  const response = await fetch(
    `/api/enrollment/${encodeURIComponent(tenantSlug)}/submit`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Anmeldung konnte nicht übermittelt werden",
    );
  }
  return readJSON<SubmitEnrollmentResult>(response);
}

export interface StatusChild {
  id: string;
  first_name: string;
  last_name: string;
  status:
    | "submitted"
    | "under_review"
    | "approved"
    | "waitlisted"
    | "rejected"
    | "withdrawn"
    // Annual phase rollover (migration 1.15.62). pending_renewal +
    // auto_renewed surface to parents; pending_admin_review is admin-only
    // and doesn't ship a status URL — but we include it in the union
    // so the type is exhaustive across all DB values.
    | "pending_renewal"
    | "auto_renewed"
    | "pending_admin_review";
  status_reason?: string | null;
}

export interface StatusResponse {
  request_id: string;
  guardian_first_name: string;
  guardian_last_name: string;
  guardian_email: string;
  guardian_phone?: string | null;
  submitted_at: string;
  withdrawn_at?: string | null;
  children: StatusChild[];
}

/**
 * Loads the status payload for a token. Returns null when the token is
 * unknown or expired (404).
 */
export async function fetchStatus(
  token: string,
): Promise<StatusResponse | null> {
  const response = await fetch(
    `/api/enrollment/requests/${encodeURIComponent(token)}`,
    { cache: "no-store" },
  );
  if (response.status === 404) return null;
  if (!response.ok) {
    throw await readError(response, "Status konnte nicht geladen werden");
  }
  return readJSON<StatusResponse>(response);
}

export async function patchStatus(
  token: string,
  patch: {
    guardian_first_name?: string;
    guardian_last_name?: string;
    guardian_phone?: string;
    consent_flags?: Record<string, unknown>;
    custom_data?: Record<string, unknown>;
  },
): Promise<void> {
  const response = await fetch(
    `/api/enrollment/requests/${encodeURIComponent(token)}`,
    {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(patch),
    },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Änderungen konnten nicht gespeichert werden",
    );
  }
}

export async function withdrawStatus(
  token: string,
  childID?: string,
): Promise<void> {
  const body = childID ? { child_id: childID } : {};
  const response = await fetch(
    `/api/enrollment/requests/${encodeURIComponent(token)}/withdraw`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    },
  );
  if (response.status === 204) return;
  if (!response.ok) {
    throw await readError(response, "Rücknahme nicht möglich");
  }
}

/**
 * Confirms an opt_in rollover: every pending_renewal child under the
 * request transitions to submitted, where the admin's review queue
 * picks them up. Returns the number of children promoted.
 */
export async function confirmRenewal(token: string): Promise<number> {
  const response = await fetch(
    `/api/enrollment/requests/${encodeURIComponent(token)}/confirm-renewal`,
    { method: "POST", headers: { "Content-Type": "application/json" } },
  );
  if (!response.ok) {
    throw await readError(response, "Bestätigung nicht möglich");
  }
  const payload = (await response.json()) as { confirmed?: number };
  return payload.confirmed ?? 0;
}
