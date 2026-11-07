import type { SignatureRequestBody } from "~/lib/signature";
import { createLogger } from "~/lib/logger";
import { downloadBlob, filenameFromDisposition } from "~/lib/file-download";
import type {
  PublicCaptchaConfig,
  PublicFormSchema,
  PublicLegalTexts,
} from "~/lib/enrollment-form-schema-api";
import type { CareOfferingAvailabilityRule } from "~/lib/care-offering-api";
import type { Translations } from "~/lib/enrollment-translations";
import {
  enrollmentErrorReader,
  readEnrollmentJSON,
  sendEnrollmentRequest,
  withLateInviteQuery,
} from "~/lib/enrollment-http";

import type { PhaseAudience } from "~/lib/enrollment-phase-api";
import type { ChangeRequestStatus, ChildStatus } from "~/lib/enrollment-status";

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
  selection_rule?:
    | "optional"
    | "exactly_one"
    | "at_least_one"
    | "at_most_one"
    | "at_most_one_per_day";
  /** Translations of `name` and `description` that still match (#3377). */
  translations?: Translations;
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
  /**
   * The form version the page showed. The server stores it beside the
   * version in force, so the evidence shows the parent ticked exactly these
   * legal texts (#4680).
   */
  form_schema_id?: number;
  custom_data?: Record<string, unknown>;
  children: SubmitChildPayload[];
  captcha_token?: string;
  late_invite_token?: string;
  /** The parent's signature when the form requires one (#4786). */
  signature?: SignatureRequestBody;
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
  /** Taken over into care: the change form shows this child read-only. */
  locked?: boolean;
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
  status: ChangeRequestStatus;
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

const readError = enrollmentErrorReader(
  logger,
  "enrollment_submission_api_failed",
);

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
  /**
   * Server-authoritative "does grade 1 declare a concrete class here?" (#1663).
   * Grade 1 is opt-in (#1833) and the form cannot derive the answer from the
   * pick list: a phase restricted to a prefixless class ("Bienen") collects it
   * for grade 1 too, yet its narrowed list looks like an unrestricted phase
   * that merely offers a named class. Optional so a payload from a backend
   * that does not send it keeps the old prefix-only rule.
   */
  collect_grade_1?: boolean;
}

export const EMPTY_SCHOOL_CLASS_CONFIG: PublicSchoolClassConfig = {
  collect: false,
  available_classes: [],
  require: false,
  collect_grade_1: false,
};

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
  /**
   * Applicant restriction (#1663). "open" (or missing) = anyone may apply;
   * "new_students" = only children not already enrolled at the school;
   * "existing_students" = the inverse, only children already enrolled may
   * apply (re-enrollment / renewal) — the public picker labels both so
   * anonymous parents see the restriction before filling the form.
   * "linked_parents" phases never reach the public picker (filtered
   * server-side).
   */
  audience?: PhaseAudience;
  /**
   * Grade restriction (#1663). Empty (or missing) = no restriction;
   * otherwise only children in one of these grades may apply. The form
   * narrows its grade select to these values so a parent never fills in
   * the whole form only to be rejected with grade_not_eligible.
   */
  eligible_grade_levels?: number[];
  /** Translations of `name` that still match the German name (#3377). */
  translations?: Translations;
}

/**
 * Fetches the currently-open phases for a tenant. Public route — no
 * JWT required. The parent landing page uses this for the phase
 * picker; clicking a phase navigates the parent into the form.
 */
export async function fetchPublicPhases(
  tenantSlug: string,
): Promise<PublicPhase[]> {
  const response = await sendEnrollmentRequest(
    `/api/enrollment/phases/public/${encodeURIComponent(tenantSlug)}`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw await readError(response, "Phasen konnten nicht geladen werden");
  }
  const list = await readEnrollmentJSON<PublicPhase[]>(response);
  return Array.isArray(list) ? list : [];
}

export interface MeProfileChild {
  id: string;
  first_name: string;
  last_name: string;
  school_class: string;
  grade_level?: number;
  /**
   * Whether this guardian relationship grants
   * `parent_portal.enrollment.submit` for this child (#1663). The reuse
   * picker only offers children where this is true — a portal-visible but
   * submit-revoked child would otherwise pass the picker and 403 at submit.
   * Optional for backward compatibility with older payloads (treated as
   * false when absent).
   */
  enrollment_submit?: boolean;
  /**
   * Lifecycle status of the linked student (`active` | `pending` |
   * `inactive`). Only active/pending children count as enrolled, so the reuse
   * picker offers no other status — an inactive child would be adopted into
   * the form and then rejected by the existing_students submit gate (#1663).
   * Optional for backward compatibility with older payloads (an absent status
   * is not proof of enrollment, so it is treated as not reusable).
   */
  status?: string;
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

export interface PublicLateInvitePrefill {
  guardian_email: string;
  guardian_first_name?: string;
  guardian_last_name?: string;
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
  late_invite?: PublicLateInvitePrefill;
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
  const response = await sendEnrollmentRequest(
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
  return readEnrollmentJSON<PublicEnrollmentBootstrap>(response);
}

/**
 * Authenticated parents-portal form-load bootstrap. Hits the parent-scoped
 * endpoint (parent NextAuth session forwarded by the proxy) instead of the
 * anonymous public path, so audience-restricted (linked_parents) phases —
 * which the public endpoint refuses (#1663) — load for a logged-in
 * guardian. The response shape matches the public bootstrap (captcha is
 * empty; the parent JWT is the anti-bot signal).
 */
export async function fetchParentEnrollmentBootstrap(
  tenantSlug: string,
  phaseId: string,
  options: LateInviteFetchOptions = {},
): Promise<PublicEnrollmentBootstrap> {
  let path = `/api/parent/enrollments/${encodeURIComponent(
    tenantSlug,
  )}/bootstrap/${encodeURIComponent(phaseId)}`;
  path = withLateInviteQuery(path, options.lateInviteToken);
  const response = await sendEnrollmentRequest(path, { cache: "no-store" });
  if (!response.ok) {
    throw await readError(
      response,
      "Anmeldeformular konnte nicht geladen werden",
    );
  }
  return readEnrollmentJSON<PublicEnrollmentBootstrap>(response);
}

/**
 * Submits an enrollment for the given tenant slug. The backend handles
 * captcha verification, schema pinning, and outbox enqueueing.
 */
export async function submitEnrollment(
  tenantSlug: string,
  payload: SubmitEnrollmentPayload,
): Promise<SubmitEnrollmentResult> {
  const response = await sendEnrollmentRequest(
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
  return readEnrollmentJSON<SubmitEnrollmentResult>(response);
}

export interface StatusChild {
  id: string;
  first_name: string;
  last_name: string;
  status: ChildStatus;
  status_reason?: string | null;
  /**
   * True once the child is taken over into care. Its data stays readable
   * through the status link, but changes run through the parents app only
   * (ADR 0003).
   */
  locked?: boolean;
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
  /**
   * Whether the primary guardian can log in to the parents app. Sent only
   * while a child is taken over into care, and left out when the backend
   * could not check it.
   */
  has_parent_account?: boolean;
  /**
   * The useful next step for the primary guardian. The backend leaves this
   * out when it could not determine the account state.
   */
  parent_portal_access?: "account" | "invitation" | "contact_ogs";
  /**
   * The form asks the parent to sign every edit (#4786); the page then
   * offers only the full form, which carries the signature field.
   */
  signature_required?: boolean;
  /** The signature of the current version, without the image. */
  signature?: EnrollmentSignature;
}

/**
 * A stored signature of an enrollment (#4786): a simple electronic
 * signature with a drawn signature. `image_png` (base64, no `data:` prefix)
 * reaches only the staff detail.
 */
export interface EnrollmentSignature {
  method: "drawn" | "typed";
  typed_name?: string;
  signer_name: string;
  signed_at: string;
  document_sha256: string;
  integrity_ok: boolean;
  image_png?: string;
}

/**
 * Loads the status payload for a token. Returns null when the token is
 * unknown or expired (404).
 */
export async function fetchStatus(
  token: string,
): Promise<StatusResponse | null> {
  const response = await sendEnrollmentRequest(
    `/api/enrollment/requests/${encodeURIComponent(token)}`,
    { cache: "no-store" },
  );
  if (response.status === 404) return null;
  if (!response.ok) {
    throw await readError(response, "Status konnte nicht geladen werden");
  }
  return readEnrollmentJSON<StatusResponse>(response);
}

/** Downloads the proof of the parent's signature behind the status link. */
export async function downloadEnrollmentSignatureProof(
  token: string,
): Promise<void> {
  const response = await sendEnrollmentRequest(
    `/api/enrollment/requests/${encodeURIComponent(token)}/signature-proof`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw await readError(response, "Nachweis konnte nicht geladen werden");
  }
  downloadBlob(
    await response.blob(),
    filenameFromDisposition(response) ?? "anmeldenachweis.pdf",
  );
}

export async function fetchEnrollmentEditBootstrap(
  token: string,
  fallback = "Anmeldung kann nicht bearbeitet werden",
): Promise<EnrollmentEditBootstrap> {
  const response = await sendEnrollmentRequest(
    `/api/enrollment/requests/${encodeURIComponent(token)}/edit-bootstrap`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw await readError(response, fallback);
  }
  return readEnrollmentJSON<EnrollmentEditBootstrap>(response);
}

export async function updateEnrollmentRequest(
  token: string,
  payload: SubmitEnrollmentPayload,
  fallback = "Änderungen konnten nicht gespeichert werden",
): Promise<SubmitEnrollmentResult> {
  const response = await sendEnrollmentRequest(
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
  return readEnrollmentJSON<SubmitEnrollmentResult>(response);
}

export async function createEnrollmentChangeRequest(
  token: string,
  payload: SubmitEnrollmentPayload,
  parentNote = "",
  fallback = "Änderungsanfrage konnte nicht gespeichert werden",
): Promise<EnrollmentChangeRequest> {
  const response = await sendEnrollmentRequest(
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
  return readEnrollmentJSON<EnrollmentChangeRequest>(response);
}

export async function listEnrollmentChangeRequests(
  token: string,
): Promise<EnrollmentChangeRequest[]> {
  const response = await sendEnrollmentRequest(
    `/api/enrollment/requests/${encodeURIComponent(token)}/change-requests`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Änderungsanfragen konnten nicht geladen werden",
    );
  }
  const list = await readEnrollmentJSON<EnrollmentChangeRequest[]>(response);
  return Array.isArray(list) ? list : [];
}

export async function replyEnrollmentChangeRequest(
  token: string,
  changeRequestId: string,
  body: string,
): Promise<EnrollmentChangeRequest> {
  const response = await sendEnrollmentRequest(
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
  return readEnrollmentJSON<EnrollmentChangeRequest>(response);
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
  const response = await sendEnrollmentRequest(
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
  const response = await sendEnrollmentRequest(
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
  const response = await sendEnrollmentRequest(
    `/api/enrollment/requests/${encodeURIComponent(token)}/confirm-renewal`,
    { method: "POST", headers: { "Content-Type": "application/json" } },
  );
  if (!response.ok) {
    throw await readError(response, "Bestätigung nicht möglich");
  }
  const payload = (await response.json()) as { confirmed?: number };
  return payload.confirmed ?? 0;
}
