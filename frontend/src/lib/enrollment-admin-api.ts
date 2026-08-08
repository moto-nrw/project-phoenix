import { createLogger } from "~/lib/logger";
import { readEnrollmentError } from "~/lib/enrollment-error-messages";
import type {
  PublicEnrollmentBootstrap,
  SubmitEnrollmentPayload,
  SubmitEnrollmentResult,
} from "~/lib/enrollment-submission-api";

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
  "approved" | "waitlisted" | "rejected" | "under_review";

export type EnrollmentRequestExportFormat = "pdf" | "docx" | "xlsx";

export type AdminEnrollmentChangeRequestStatus =
  | "pending_review"
  | "needs_parent_response"
  | "approved"
  | "rejected"
  | "cancelled";

type AdminEnrollmentChangeRequestMessageAuthor = "parent" | "staff" | "system";

export interface AdminRequestChildOffering {
  offering_id: string;
  offering_name: string;
  days_of_week_mode: string; // "fixed" | "parent_choice"
  /** Parent's day picks. Present only for parent_choice offerings. */
  selected_days?: string[];
  manual_selected_days?: string[];
  automatic_selected_days?: string[];
  /**
   * Offering's full day catalogue. For fixed offerings this is the
   * authoritative schedule; for parent_choice it bounds the picker.
   */
  available_days?: string[];
  /**
   * Offering attributes as configured on the phase's care offering. The
   * parents app renders these for the same booking, so staff must see them
   * too — otherwise a guardian describes something staff cannot find (#2185).
   */
  includes_lunch?: boolean;
  includes_holiday_care?: boolean;
  price_cents?: number;
  /** ISO "YYYY-MM-DD". Set when the booking starts later than the period. */
  valid_from?: string;
  /**
   * ISO "YYYY-MM-DD", the INCLUSIVE last day the booking covers. The backend
   * converts its exclusive column for the wire, same as the parent endpoint.
   */
  valid_until?: string;
  /**
   * True for a booking that has not taken effect yet (an approved change
   * scheduled for a future date, or any booking while the phase is still
   * ahead). Display only — which ARRAY a row arrives in says whether a
   * correction replaces it, see `upcoming_offerings` on the child.
   */
  starts_later?: boolean;
}

interface AdminOfferingAdjustmentSnapshot {
  offering_id: string;
  offering_name?: string;
  days_of_week_mode?: string;
  selected_days?: string[];
  manual_selected_days?: string[];
  automatic_selected_days?: string[];
  available_days?: string[];
}

export interface AdminOfferingAdjustment {
  id: string;
  request_id: string;
  request_child_id: string;
  student_id: string;
  actor_account_id: string;
  actor_role: string;
  actor_name_snapshot?: string | null;
  actor_email_snapshot?: string | null;
  reason: string;
  before: AdminOfferingAdjustmentSnapshot[];
  after: AdminOfferingAdjustmentSnapshot[];
  changed_at: string;
}

interface AdminEnrollmentChangeRequestMessage {
  id: string;
  author_type: AdminEnrollmentChangeRequestMessageAuthor;
  author_account_id?: number | null;
  body: string;
  internal_only: boolean;
  created_at: string;
}

export interface AdminEnrollmentChangeRequest {
  id: string;
  request_id: string;
  origin: "parent" | "admin";
  status: AdminEnrollmentChangeRequestStatus;
  parent_note?: string | null;
  admin_decision_note?: string | null;
  base_snapshot: Record<string, unknown>;
  proposed_snapshot: Record<string, unknown>;
  diff: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  reviewed_at?: string | null;
  reviewed_by_account_id?: number | null;
  request?: AdminRequestSummary;
  messages?: AdminEnrollmentChangeRequestMessage[];
}

export interface AdminChildDataCorrectionResult extends AdminEnrollmentChangeRequest {
  request: AdminRequestSummary;
}

export interface UpdateAdminChildOfferingsInput {
  offerings: Array<{ offering_id: string; selected_days?: string[] }>;
  reason: string;
}

export interface CorrectAdminChildDataInput {
  first_name: string;
  last_name: string;
  date_of_birth: string;
  target_grade_level?: number;
  target_school_class?: string;
  reason: string;
}

export interface AdminRequestChild {
  id: string;
  first_name: string;
  last_name: string;
  date_of_birth: string;
  target_grade_level?: number;
  /** Concrete future class (e.g. "2a") when collected (#1833). */
  target_school_class?: string;
  status: ChildStatus;
  status_reason?: string | null;
  reviewed_at?: string | null;
  reviewed_by?: number | null;
  activation_mode: string;
  created_student_id?: string;
  custom_data?: Record<string, unknown>;
  /**
   * The Betreuungsangebote selection on file RIGHT NOW — exactly what a
   * correction replaces. Detail endpoint only.
   */
  offerings?: AdminRequestChildOffering[];
  /**
   * Bookings that only take effect on a later date. Display only: never
   * feed these into a correction payload, or an untouched save applies a
   * future change months early (#2185).
   */
  upcoming_offerings?: AdminRequestChildOffering[];
  /**
   * True when the booking lookup FAILED — as opposed to the child having
   * no bookings. An empty editor plus a save deletes what the family
   * actually booked, so corrections must be blocked in this state.
   */
  offerings_unavailable?: boolean;
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
  /**
   * key→title pairs from the pinned schema's legal blocks, used to label
   * custom consent flags instead of rendering raw keys. Detail endpoint
   * only.
   */
  schema_legal_blocks?: Array<{ key: string; title: string }>;
  children: AdminRequestChild[];
  /**
   * Co-guardians the parent added beyond the primary guardian above.
   * Empty/absent when none were added.
   */
  additional_guardians?: AdminRequestGuardian[];
}

export interface AdminRequestDetail extends AdminRequestSummary {
  status_token: string;
  late_invite_guardian_email?: string;
  late_invite_email_mismatch?: boolean;
}

interface AdminEnrollmentDeletionCounts {
  requests: number;
  request_children: number;
  request_child_offerings: number;
  request_guardians: number;
  change_requests: number;
  change_request_messages: number;
  late_invites: number;
  offering_adjustments: number;
  email_outbox: number;
  rollover_links_cleared: number;
  student_source_links_cleared: number;
  total: number;
}

export interface AdminEnrollmentDeletionImpact {
  request_id: string;
  child_id?: string;
  deletes_request: boolean;
  can_delete: boolean;
  counts: AdminEnrollmentDeletionCounts;
  blocking_student_ids: string[];
  preserved_guardian_profiles: number;
  preserved_parent_accounts: number;
  unlinked_guardian_profiles: number;
  parent_accounts_without_students: number;
}

/** One additional guardian (co-guardian) on a submission. */
export interface AdminRequestGuardian {
  id: string;
  first_name: string;
  last_name: string;
  email?: string | null;
  phone?: string | null;
}

interface BackendEnvelope<T> {
  status?: string;
  data?: T;
  error?: string;
  message?: string;
}

const BASE = "/api/enrollment/admin/requests";
const CHANGE_REQUEST_BASE = "/api/enrollment/admin/change-requests";
const PHASE_BASE = "/api/enrollment/phases";

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
  return readEnrollmentError(
    response,
    fallback,
    logger,
    "enrollment_admin_request_failed",
  );
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

export async function getAdminRequest(id: string): Promise<AdminRequestDetail> {
  const response = await fetch(`${BASE}/${encodeURIComponent(id)}`, {
    cache: "no-store",
  });
  if (!response.ok) {
    throw await readError(response, "Anmeldung konnte nicht geladen werden");
  }
  return readJSON<AdminRequestDetail>(response);
}

export async function getAdminRequestDeleteImpact(
  requestId: string,
): Promise<AdminEnrollmentDeletionImpact> {
  const response = await fetch(
    `${BASE}/${encodeURIComponent(requestId)}/delete-impact`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Auswirkungen der Löschung konnten nicht geladen werden",
    );
  }
  return readJSON<AdminEnrollmentDeletionImpact>(response);
}

export async function getAdminChildDeleteImpact(
  requestId: string,
  childId: string,
): Promise<AdminEnrollmentDeletionImpact> {
  const response = await fetch(
    `${BASE}/${encodeURIComponent(requestId)}/children/${encodeURIComponent(childId)}/delete-impact`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Auswirkungen der Löschung konnten nicht geladen werden",
    );
  }
  return readJSON<AdminEnrollmentDeletionImpact>(response);
}

export async function deleteAdminRequest(
  requestId: string,
  reason: string,
): Promise<AdminEnrollmentDeletionImpact> {
  const response = await fetch(`${BASE}/${encodeURIComponent(requestId)}`, {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ reason }),
  });
  if (!response.ok) {
    throw await readError(response, "Anmeldung konnte nicht gelöscht werden");
  }
  return readJSON<AdminEnrollmentDeletionImpact>(response);
}

export interface AdminRestoreResult {
  restored_children: number;
  /**
   * Subset of restored_children that came back as "waitlisted" because an
   * offering is meanwhile full (the capacity gate re-runs on restore).
   */
  waitlisted_children: number;
}

/**
 * Restores a withdrawn request: every withdrawn child goes back to
 * "submitted" — or "waitlisted" when its offering is meanwhile full —
 * and withdrawn_at is cleared (#2157). Fails with a coded 409 when the
 * phase is inactive, an active duplicate request exists, or a reject-mode
 * phase's offering is full.
 */
export async function restoreAdminRequest(
  requestId: string,
): Promise<AdminRestoreResult> {
  const response = await fetch(
    `${BASE}/${encodeURIComponent(requestId)}/restore`,
    { method: "POST" },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Anmeldung konnte nicht wiederhergestellt werden",
    );
  }
  return readJSON<AdminRestoreResult>(response);
}

export async function deleteAdminChild(
  requestId: string,
  childId: string,
  reason: string,
): Promise<AdminEnrollmentDeletionImpact> {
  const response = await fetch(
    `${BASE}/${encodeURIComponent(requestId)}/children/${encodeURIComponent(childId)}`,
    {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ reason }),
    },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Kind konnte nicht aus der Anmeldung gelöscht werden",
    );
  }
  return readJSON<AdminEnrollmentDeletionImpact>(response);
}

export interface CreateLateInviteInput {
  guardian_email: string;
  reason?: string;
  expires_at?: string;
}

export interface CreateLateInviteResult {
  id: string;
  phase_id: string;
  guardian_email: string;
  guardian_first_name?: string | null;
  guardian_last_name?: string | null;
  expires_at: string;
  created_by: string;
  reason?: string | null;
  token: string;
}

export async function createLateInvite(
  phaseId: string,
  input: CreateLateInviteInput,
): Promise<CreateLateInviteResult> {
  const response = await fetch(
    `${PHASE_BASE}/${encodeURIComponent(phaseId)}/late-invites`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Nachzügler-Link konnte nicht erstellt werden",
    );
  }
  return readJSON<CreateLateInviteResult>(response);
}

export async function fetchManualEnrollmentBootstrap(
  phaseId: string,
): Promise<PublicEnrollmentBootstrap> {
  const response = await fetch(
    `${PHASE_BASE}/${encodeURIComponent(phaseId)}/manual-bootstrap`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Formularvorlage konnte nicht geladen werden",
    );
  }
  return readJSON<PublicEnrollmentBootstrap>(response);
}

export interface CreateManualApprovedEnrollmentInput extends SubmitEnrollmentPayload {
  external_consent_confirmed: boolean;
  reason: string;
  send_notification: boolean;
}

export async function createManualApprovedEnrollment(
  phaseId: string,
  input: CreateManualApprovedEnrollmentInput,
): Promise<SubmitEnrollmentResult> {
  const response = await fetch(
    `${PHASE_BASE}/${encodeURIComponent(phaseId)}/manual-approved-enrollments`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Manuelle Anmeldung konnte nicht freigegeben werden",
    );
  }
  return readJSON<SubmitEnrollmentResult>(response);
}

export async function listStudentEnrollmentRequests(
  studentId: string,
): Promise<AdminRequestSummary[]> {
  const response = await fetch(
    `/api/enrollment/admin/students/${encodeURIComponent(studentId)}/requests`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw await readError(response, "Anmeldungen konnten nicht geladen werden");
  }
  const list = await readJSON<AdminRequestSummary[]>(response);
  return Array.isArray(list) ? list : [];
}

export async function exportStudentEnrollmentRequests(
  studentId: string,
  format: EnrollmentRequestExportFormat,
): Promise<void> {
  const response = await fetch(
    `/api/enrollment/admin/students/${encodeURIComponent(studentId)}/requests/export`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ format }),
    },
  );

  if (!response.ok) {
    throw await readError(response, "Export konnte nicht erstellt werden");
  }

  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filenameFromDisposition(response) ?? `anmeldungen.${format}`;
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
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

export async function updateAdminChildOfferings(
  requestId: string,
  childId: string,
  input: UpdateAdminChildOfferingsInput,
): Promise<AdminRequestChild> {
  const response = await fetch(`/api/enrollment/admin/offerings`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      request_id: requestId,
      child_id: childId,
      offerings: input.offerings,
      reason: input.reason,
    }),
  });
  if (!response.ok) {
    throw await readError(
      response,
      "Betreuungsangebote konnten nicht gespeichert werden",
    );
  }
  return readJSON<AdminRequestChild>(response);
}

export async function correctAdminChildData(
  requestId: string,
  childId: string,
  input: CorrectAdminChildDataInput,
): Promise<AdminChildDataCorrectionResult> {
  const response = await fetch(`/api/enrollment/admin/data-correction`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      request_id: requestId,
      child_id: childId,
      ...input,
    }),
  });
  if (!response.ok) {
    throw await readError(
      response,
      "Anmeldedaten konnten nicht korrigiert werden",
    );
  }
  return readJSON<AdminChildDataCorrectionResult>(response);
}

export async function listAdminChildOfferingAdjustments(
  requestId: string,
  childId: string,
): Promise<AdminOfferingAdjustment[]> {
  const url = new URL(
    "/api/enrollment/admin/offering-adjustments",
    globalThis.window?.location.origin ?? "http://localhost",
  );
  url.searchParams.set("request_id", requestId);
  url.searchParams.set("child_id", childId);
  const response = await fetch(`${url.pathname}${url.search}`, {
    cache: "no-store",
  });
  if (!response.ok) {
    throw await readError(
      response,
      "Änderungshistorie konnte nicht geladen werden",
    );
  }
  const list = await readJSON<AdminOfferingAdjustment[]>(response);
  return Array.isArray(list) ? list : [];
}

export interface ListAdminEnrollmentChangeRequestsFilters {
  requestId?: string;
  status?: AdminEnrollmentChangeRequestStatus;
}

export async function listAdminEnrollmentChangeRequests(
  filters: ListAdminEnrollmentChangeRequestsFilters = {},
): Promise<AdminEnrollmentChangeRequest[]> {
  const url = new URL(
    CHANGE_REQUEST_BASE,
    globalThis.window?.location.origin ?? "http://localhost",
  );
  if (filters.requestId) url.searchParams.set("request_id", filters.requestId);
  if (filters.status) url.searchParams.set("status", filters.status);
  const response = await fetch(`${url.pathname}${url.search}`, {
    cache: "no-store",
  });
  if (!response.ok) {
    throw await readError(
      response,
      "Änderungsanfragen konnten nicht geladen werden",
    );
  }
  const list = await readJSON<AdminEnrollmentChangeRequest[]>(response);
  return Array.isArray(list) ? list : [];
}

export async function getAdminEnrollmentChangeRequest(
  id: string,
): Promise<AdminEnrollmentChangeRequest> {
  const response = await fetch(
    `${CHANGE_REQUEST_BASE}/${encodeURIComponent(id)}`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Änderungsanfrage konnte nicht geladen werden",
    );
  }
  return readJSON<AdminEnrollmentChangeRequest>(response);
}

export async function askEnrollmentChangeRequestQuestion(
  id: string,
  body: string,
): Promise<AdminEnrollmentChangeRequest> {
  const response = await fetch(
    `${CHANGE_REQUEST_BASE}/${encodeURIComponent(id)}/question`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ body }),
    },
  );
  if (!response.ok) {
    throw await readError(response, "Rückfrage konnte nicht gesendet werden");
  }
  return readJSON<AdminEnrollmentChangeRequest>(response);
}

export async function approveEnrollmentChangeRequest(
  id: string,
  note: string,
): Promise<AdminEnrollmentChangeRequest> {
  const response = await fetch(
    `${CHANGE_REQUEST_BASE}/${encodeURIComponent(id)}/approve`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ note }),
    },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Änderungsanfrage konnte nicht freigegeben werden",
    );
  }
  return readJSON<AdminEnrollmentChangeRequest>(response);
}

export async function rejectEnrollmentChangeRequest(
  id: string,
  note: string,
): Promise<AdminEnrollmentChangeRequest> {
  const response = await fetch(
    `${CHANGE_REQUEST_BASE}/${encodeURIComponent(id)}/reject`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ note }),
    },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Änderungsanfrage konnte nicht abgelehnt werden",
    );
  }
  return readJSON<AdminEnrollmentChangeRequest>(response);
}

function filenameFromDisposition(response: Response): string | null {
  const disposition = response.headers.get("content-disposition");
  const match = /filename="([^"]+)"/.exec(disposition ?? "");
  return match?.[1] ?? null;
}
