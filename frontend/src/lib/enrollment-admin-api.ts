import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentAdminAPI" });

export type ChildStatus =
  | "submitted"
  | "under_review"
  | "approved"
  | "waitlisted"
  | "rejected"
  | "withdrawn"
  | "pending_renewal"
  | "auto_renewed"
  | "pending_admin_review";

export type DecisionStatus =
  | "approved"
  | "waitlisted"
  | "rejected"
  | "under_review";

export interface AdminRequestChild {
  id: string;
  first_name: string;
  last_name: string;
  date_of_birth: string;
  target_grade_level?: number;
  status: ChildStatus;
  status_reason?: string | null;
  reviewed_at?: string | null;
  reviewed_by?: number | null;
  activation_mode: string;
  custom_data?: Record<string, unknown>;
}

/** Slim form-field shape, only what the admin detail UI needs. */
export interface AdminRequestSchemaField {
  key: string;
  label: string;
  type: string;
  applies_to_child: boolean;
  target?: string;
  /**
   * For select-type fields: the {label, value} pairs the admin
   * defined. The renderer turns the stored value back into the
   * label (e.g. "picked_up" → "Wird abgeholt").
   */
  options?: Array<{ label: string; value: string }>;
}

export interface AdminRequestSummary {
  id: string;
  phase_id: string;
  phase_name: string;
  guardian_first_name: string;
  guardian_last_name: string;
  guardian_email: string;
  guardian_phone?: string | null;
  submitted_at: string;
  withdrawn_at?: string | null;
  status_token: string;
  /**
   * Request-level custom field answers (everything where
   * applies_to_child=false). Populated only on the detail endpoint.
   */
  custom_data?: Record<string, unknown>;
  /** AGB / Datenschutz / Foto consents collected on the base form. */
  consent_flags?: Record<string, unknown>;
  /**
   * The form schema's field definitions, used to render custom_data
   * with their original labels. Populated only on the detail endpoint;
   * empty when the phase didn't pin a schema.
   */
  schema_fields?: AdminRequestSchemaField[];
  children: AdminRequestChild[];
}

interface BackendEnvelope<T> {
  status?: string;
  data?: T;
  error?: string;
  message?: string;
}

const BASE = "/api/enrollment/admin/requests";

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
  logger.error("enrollment_admin_request_failed", {
    status: response.status,
    message,
  });
  const err = new Error(message) as Error & { status?: number };
  err.status = response.status;
  return err;
}

export interface ListAdminRequestsFilters {
  phaseId?: string;
  childStatus?: ChildStatus;
}

export async function listAdminRequests(
  filters: ListAdminRequestsFilters = {},
): Promise<AdminRequestSummary[]> {
  const url = new URL(
    BASE,
    globalThis.window?.location.origin ?? "http://localhost",
  );
  if (filters.phaseId) url.searchParams.set("phase_id", filters.phaseId);
  if (filters.childStatus)
    url.searchParams.set("child_status", filters.childStatus);
  const path = `${url.pathname}${url.search}`;
  const response = await fetch(path, { cache: "no-store" });
  if (!response.ok) {
    throw await readError(response, "Anmeldungen konnten nicht geladen werden");
  }
  const list = await readJSON<AdminRequestSummary[]>(response);
  return Array.isArray(list) ? list : [];
}

export async function getAdminRequest(
  id: string,
): Promise<AdminRequestSummary> {
  const response = await fetch(`${BASE}/${encodeURIComponent(id)}`, {
    cache: "no-store",
  });
  if (!response.ok) {
    throw await readError(response, "Anmeldung konnte nicht geladen werden");
  }
  return readJSON<AdminRequestSummary>(response);
}

export async function decideAdminChild(
  requestId: string,
  childId: string,
  status: DecisionStatus,
  reason?: string,
): Promise<AdminRequestChild> {
  // Flattened path: the proxy reads request_id/child_id from the body.
  // The deep dynamic /requests/[id]/children/[childId]/decide path
  // hits a Turbopack dev bug where the route disappears after cache
  // compaction; this collapsed shape keeps Next dev stable.
  const response = await fetch(`/api/enrollment/admin/decide`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      request_id: requestId,
      child_id: childId,
      status,
      reason: reason ?? "",
    }),
  });
  if (!response.ok) {
    throw await readError(
      response,
      "Entscheidung konnte nicht gespeichert werden",
    );
  }
  return readJSON<AdminRequestChild>(response);
}
