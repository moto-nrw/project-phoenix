import { createLogger } from "~/lib/logger";
import { readEnrollmentError } from "~/lib/enrollment-error-messages";
import type {
  PublicCaptchaConfig,
  PublicFormSchema,
  PublicLegalTexts,
} from "~/lib/enrollment-form-schema-api";
import type { CareOfferingAvailabilityRule } from "~/lib/care-offering-api";

const logger = createLogger({ component: "EnrollmentSubmissionAPI" });

export type CareOfferingSelectionMode =
  "optional" | "at_least_one" | "exactly_one";

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
  is_required: boolean;
  counts_as_care?: boolean;
  auto_add_grade_levels?: number[];
  availability_rule?: CareOfferingAvailabilityRule | null;
  auto_add_trigger_offering_ids?: string[];
  /**
   * Offerings sharing a non-empty selection_group are constrained
   * together by selection_rule (exactly_one / at_least_one /
   * at_most_one). Empty group = ungrouped, no constraint.
   */
  selection_group?: string | null;
  selection_rule?: "optional" | "exactly_one" | "at_least_one" | "at_most_one";
}

interface SubmitOfferingDays {
  offering_id: number;
  selected_days: string[];
}

export interface SubmitChildPayload {
  id?: string;
  first_name: string;
  last_name: string;
  date_of_birth: string; // YYYY-MM-DD
  target_grade_level?: number;
  /**
   * Concrete future class (e.g. "2a"), collected from grade 2 upwards
   * when the tenant enables it (#1833). Omitted for grade 1 or when the
   * parent leaves it open ("Klasse offen").
   */
  target_school_class?: string;
  custom_data?: Record<string, unknown>;
  offering_ids?: number[];
  /**
   * Per-offering day picks for offerings whose days_of_week_mode is
   * "parent_choice". Omitted entirely when no such offerings are
   * selected. Each entry's offering_id must also be in offering_ids.
   */
  offering_days?: SubmitOfferingDays[];
}

/**
 * One additional guardian (co-guardian) the parent added beyond the
 * primary guardian. Names are required; email and phone are optional —
 * a co-guardian (e.g. a grandparent) may be a contact-only record.
 */
export interface SubmitGuardianPayload {
  first_name: string;
  last_name: string;
  email?: string;
  phone?: string;
}

export interface SubmitEnrollmentPayload {
  phase_id: number;
  guardian_first_name: string;
  guardian_last_name: string;
  guardian_email: string;
  guardian_phone?: string;
  /**
   * Co-guardians beyond the primary guardian. Omitted (or empty) when
   * the parent added none. On approval each becomes an additional
   * users.students_guardians link to every enrolled child.
   */
  additional_guardians?: SubmitGuardianPayload[];
  consent_flags?: Record<string, unknown>;
  custom_data?: Record<string, unknown>;
  children: SubmitChildPayload[];
  captcha_token?: string;
  late_invite_token?: string;
}

export interface SubmitEnrollmentResult {
  request_id: string;
  status_url: string;
  warnings?: Array<{ code: string }>;
}

interface EnrollmentEditDraftOfferingDays {
  offering_id: string;
  selected_days: string[];
  manual_selected_days?: string[];
  automatic_selected_days?: string[];
}

interface EnrollmentEditDraftChild {
  id: string;
  first_name: string;
  last_name: string;
  date_of_birth: string;
  target_grade_level?: number;
  target_school_class?: string;
  custom_data?: Record<string, unknown>;
  offering_ids: string[];
  offering_days?: EnrollmentEditDraftOfferingDays[];
}

interface EnrollmentEditDraftGuardian {
  first_name: string;
  last_name: string;
  email?: string | null;
  phone?: string | null;
}

export interface EnrollmentEditDraft {
  request_id: string;
  status_token: string;
  tenant_id: string;
  tenant_subdomain: string;
  phase_id: string;
  guardian_first_name: string;
  guardian_last_name: string;
  guardian_email: string;
  guardian_phone?: string | null;
  consent_flags: Record<string, unknown>;
  custom_data: Record<string, unknown>;
  additional_guardians?: EnrollmentEditDraftGuardian[];
  children: EnrollmentEditDraftChild[];
}

type EnrollmentEditMode = "direct_edit" | "change_request";
type EnrollmentStatusEditMode = EnrollmentEditMode | "none";

export interface EnrollmentEditBootstrap {
  edit_mode: EnrollmentEditMode;
  phase: PublicPhase;
  schema: PublicFormSchema | null;
  offerings: PublicCareOffering[];
  care_offering_selection_mode: CareOfferingSelectionMode;
  care_required: boolean;
  /** Concrete-class config (#1833) so reopened edits keep the class field. */
  school_class?: PublicSchoolClassConfig;
  collect_grade_level: boolean;
  care_offerings_enabled: boolean;
  /** Server-authoritative tenant cap (`enrollment.grade_level_max`). */
  grade_level_max: number;
  legal_texts: PublicLegalTexts;
  draft: EnrollmentEditDraft;
}

type EnrollmentChangeRequestStatus =
  | "pending_review"
  | "needs_parent_response"
  | "approved"
  | "rejected"
  | "cancelled";

type EnrollmentChangeRequestMessageAuthor = "parent" | "staff" | "system";

interface EnrollmentChangeRequestMessage {
  id: string;
  author_type: EnrollmentChangeRequestMessageAuthor;
  author_account_id?: number | null;
  body: string;
  internal_only: boolean;
  created_at: string;
}

export interface EnrollmentChangeRequest {
  id: string;
  request_id: string;
  origin: "parent" | "admin";
  status: EnrollmentChangeRequestStatus;
  parent_note?: string | null;
  admin_decision_note?: string | null;
  base_snapshot: Record<string, unknown>;
  proposed_snapshot: Record<string, unknown>;
  diff: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  reviewed_at?: string | null;
  reviewed_by_account_id?: number | null;
  messages?: EnrollmentChangeRequestMessage[];
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
  return readEnrollmentError(
    response,
    fallback,
    logger,
    "enrollment_submission_api_failed",
  );
}

/**
 * Concrete-class config for a phase (#1833): whether the tenant collects
 * a concrete class at all, the phase's pick list, and whether it is
 * mandatory from grade 2. Mirrors the backend PublicSchoolClassConfig
 * (`school_class` on the bootstrap + care-offerings responses).
 */
export interface PublicSchoolClassConfig {
  collect: boolean;
  available_classes: string[];
  require: boolean;
}

export const EMPTY_SCHOOL_CLASS_CONFIG: PublicSchoolClassConfig = {
  collect: false,
  available_classes: [],
  require: false,
};

function parseSchoolClassConfig(raw: unknown): PublicSchoolClassConfig {
  const obj = (raw ?? {}) as Partial<PublicSchoolClassConfig>;
  return {
    collect: obj.collect === true,
    available_classes: Array.isArray(obj.available_classes)
      ? obj.available_classes.filter((c): c is string => typeof c === "string")
      : [],
    require: obj.require === true,
  };
}

export interface PublicCareOfferingsResult {
  offerings: PublicCareOffering[];
  careOfferingSelectionMode: CareOfferingSelectionMode;
  careRequired: boolean;
  /**
   * Concrete-class config (#1833). Present only when the backend
   * includes `school_class` in the response; consumers coalesce to
   * EMPTY_SCHOOL_CLASS_CONFIG when absent.
   */
  schoolClass?: PublicSchoolClassConfig;
  collectGradeLevel: boolean;
  careOfferingsEnabled: boolean;
}

export interface LateInviteFetchOptions {
  lateInviteToken?: string;
}

export interface PublicEnrollmentBootstrapFetchOptions extends LateInviteFetchOptions {
  /**
   * Omits browser credentials from this request. This is transport hardening
   * for parent-portal callers, not the server-side portal boundary; the route
   * decides profile enrichment from the proxy-preserved original host.
   */
  omitCredentials?: boolean;
}

function withLateInviteQuery(path: string, token?: string): string {
  const trimmed = token?.trim();
  if (!trimmed) return path;
  return `${path}?late_invite=${encodeURIComponent(trimmed)}`;
}

/**
 * Fetches the public care-offering catalog for a given tenant slug
 * and phase. Returns the offerings plus the phase-level selection mode
 * so the form can render the hint and validate before submit. The
 * backend re-checks the mode on submit.
 */
export async function fetchPublicCareOfferings(
  tenantSlug: string,
  phaseId: string,
  options: LateInviteFetchOptions = {},
): Promise<PublicCareOfferingsResult> {
  const path = `/api/enrollment/care-offerings/public/${encodeURIComponent(
    tenantSlug,
  )}/${encodeURIComponent(phaseId)}`;
  const response = await fetch(
    withLateInviteQuery(path, options.lateInviteToken),
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Betreuungsangebote konnten nicht geladen werden",
    );
  }
  const payload = await readJSON<{
    offerings?: PublicCareOffering[];
    care_offering_selection_mode?: CareOfferingSelectionMode;
    care_required?: boolean;
    school_class?: unknown;
    collect_grade_level?: boolean;
    care_offerings_enabled?: boolean;
  }>(response);
  const mode =
    payload?.care_offering_selection_mode ??
    (payload?.care_required === true ? "at_least_one" : "optional");
  return {
    offerings: Array.isArray(payload?.offerings) ? payload.offerings : [],
    careOfferingSelectionMode: mode,
    careRequired: mode !== "optional",
    collectGradeLevel: payload?.collect_grade_level !== false,
    careOfferingsEnabled: payload?.care_offerings_enabled !== false,
    // Attach the config only when the backend actually sent it, so the
    // normalized shape for a payload without `school_class` is unchanged.
    schoolClass:
      payload?.school_class == null
        ? undefined
        : parseSchoolClassConfig(payload.school_class),
  };
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
  care_offering_selection_mode: CareOfferingSelectionMode;
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

export interface PublicEnrollmentBootstrap {
  phase: PublicPhase;
  schema: PublicFormSchema | null;
  offerings: PublicCareOffering[];
  care_offering_selection_mode: CareOfferingSelectionMode;
  care_required: boolean;
  school_class?: PublicSchoolClassConfig;
  collect_grade_level: boolean;
  care_offerings_enabled: boolean;
  captcha_config: PublicCaptchaConfig | null;
  legal_texts: PublicLegalTexts;
  profile?: MeProfileResponse | null;
}

export async function fetchPublicEnrollmentBootstrap(
  tenantSlug: string,
  phaseId: string,
  options: PublicEnrollmentBootstrapFetchOptions = {},
): Promise<PublicEnrollmentBootstrap> {
  let path = `/api/enrollment/form-bootstrap/public/${encodeURIComponent(
    tenantSlug,
  )}/${encodeURIComponent(phaseId)}`;
  path = withLateInviteQuery(path, options.lateInviteToken);
  const response = await fetch(
    path,
    options.omitCredentials
      ? { cache: "no-store", credentials: "omit" }
      : { cache: "no-store" },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Anmeldeformular konnte nicht geladen werden",
    );
  }
  return readJSON<PublicEnrollmentBootstrap>(response);
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

/** One additional guardian (co-guardian) on the public status page. */
export interface StatusGuardian {
  first_name: string;
  last_name: string;
  email?: string | null;
  phone?: string | null;
}

export interface StatusResponse {
  request_id: string;
  guardian_first_name: string;
  guardian_last_name: string;
  guardian_email: string;
  guardian_phone?: string | null;
  submitted_at: string;
  withdrawn_at?: string | null;
  edit_mode: EnrollmentStatusEditMode;
  children: StatusChild[];
  /** Co-guardians the parent added beyond the primary guardian. */
  additional_guardians?: StatusGuardian[];
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

export async function fetchEnrollmentEditBootstrap(
  token: string,
  fallback = "Anmeldung kann nicht bearbeitet werden",
): Promise<EnrollmentEditBootstrap> {
  const response = await fetch(
    `/api/enrollment/requests/${encodeURIComponent(token)}/edit-bootstrap`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw await readError(response, fallback);
  }
  return readJSON<EnrollmentEditBootstrap>(response);
}

export async function updateEnrollmentRequest(
  token: string,
  payload: SubmitEnrollmentPayload,
  fallback = "Änderungen konnten nicht gespeichert werden",
): Promise<SubmitEnrollmentResult> {
  const response = await fetch(
    `/api/enrollment/requests/${encodeURIComponent(token)}`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    },
  );
  if (!response.ok) {
    throw await readError(response, fallback);
  }
  return readJSON<SubmitEnrollmentResult>(response);
}

export async function createEnrollmentChangeRequest(
  token: string,
  payload: SubmitEnrollmentPayload,
  parentNote = "",
  fallback = "Änderungsanfrage konnte nicht gespeichert werden",
): Promise<EnrollmentChangeRequest> {
  const response = await fetch(
    `/api/enrollment/requests/${encodeURIComponent(token)}/change-requests`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        ...payload,
        parent_note: parentNote,
      }),
    },
  );
  if (!response.ok) {
    throw await readError(response, fallback);
  }
  return readJSON<EnrollmentChangeRequest>(response);
}

export async function listEnrollmentChangeRequests(
  token: string,
): Promise<EnrollmentChangeRequest[]> {
  const response = await fetch(
    `/api/enrollment/requests/${encodeURIComponent(token)}/change-requests`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Änderungsanfragen konnten nicht geladen werden",
    );
  }
  const list = await readJSON<EnrollmentChangeRequest[]>(response);
  return Array.isArray(list) ? list : [];
}

export async function replyEnrollmentChangeRequest(
  token: string,
  changeRequestId: string,
  body: string,
): Promise<EnrollmentChangeRequest> {
  const response = await fetch(
    `/api/enrollment/requests/${encodeURIComponent(
      token,
    )}/change-requests/${encodeURIComponent(changeRequestId)}/messages`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ body }),
    },
  );
  if (!response.ok) {
    throw await readError(response, "Antwort konnte nicht gesendet werden");
  }
  return readJSON<EnrollmentChangeRequest>(response);
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
