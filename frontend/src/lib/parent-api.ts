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
import type { ChatMessage, RequestDiffEntry } from "~/lib/messaging-status";
import { readEnrollmentError } from "~/lib/enrollment-error-messages";
import type {
  MeProfileResponse,
  SubmitEnrollmentPayload,
  SubmitEnrollmentResult,
} from "~/lib/enrollment-submission-api";

const logger = createLogger({ component: "ParentAPI" });

type ChildStatus = "pending" | "active" | "inactive" | "alumnus";

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
type EnrollmentChildStatus =
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

// One enrollment phase the calling parent may apply to, joined to its school.
// Mirrors api/parent EnrollablePhaseResponse. Rows are pre-filtered server-side
// (only phases the account may actually apply to) and pre-sorted (already_linked
// DESC, school name, service start). int64 ids arrive as strings.
export interface EnrollablePhase {
  readonly school_id: string;
  readonly school_name: string;
  readonly school_slug: string;
  // The school's tenant routing identifier. Enrollment links MUST be built from
  // this, not from school_slug: /auth/tenant/resolve and the parent enrollment
  // endpoints both resolve tenants by subdomain, and slug is only unique per
  // organization (#1663).
  readonly school_subdomain: string;
  readonly phase_id: string;
  readonly phase_name: string;
  readonly phase_kind: "school_year" | "holiday" | "custom";
  readonly service_start_date: string; // YYYY-MM-DD
  readonly service_end_date: string; // YYYY-MM-DD
  readonly enrollment_open_at?: string; // ISO timestamp
  readonly enrollment_close_at?: string; // ISO timestamp
  // True when the parent already has a linked child at this school.
  readonly already_linked: boolean;
  // Who the phase is open to: "open" (anyone), "new_students" (children not yet
  // enrolled), "existing_students" (only children already enrolled — a
  // re-enrollment / renewal phase), or "linked_parents" (only guardians with an
  // existing linked child). The backend can return any of these (#1663).
  readonly audience:
    "open" | "new_students" | "existing_students" | "linked_parents";
}

export interface ParentProfile {
  readonly first_name?: string;
  readonly last_name?: string;
  // null = the guardian has never picked a parents-portal language, so the
  // client keeps the anonymous cookie/Accept-Language locale.
  readonly portal_locale: AppLocale | null;
}

// The two absence kinds a parent can report: "sick" (Krankmeldung, flips the
// live sick flag) or "excused" (Termin/Abwesenheit, no live flag).
export type StudentStatusKind = "sick" | "excused";

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

// The lifecycle of a parent absence request while the OGS decision is pending
// or after it was decided. Mirrors the backend's legacy-named request states.
type ExcusedRequestStatus = "pending" | "approved" | "rejected" | "withdrawn";

// One sick or excused absence request awaiting (or having received) an OGS
// decision. Mirrors api/parent.ParentExcusedRequestResponse; the type and route
// retain their legacy names. `dates` are YYYY-MM-DD.
export interface ExcusedRequest {
  readonly id: string;
  readonly student_id: string;
  readonly absence_status: StudentStatusKind;
  readonly status: ExcusedRequestStatus;
  readonly dates: string[]; // YYYY-MM-DD
  readonly note: string;
  readonly decision_reason?: string;
  readonly created_at: string; // ISO timestamp
  readonly reviewed_at?: string; // ISO timestamp
  // is_self is true only for the calling guardian's own request. In a
  // multi-guardian family only the submitter may withdraw it (the backend
  // rejects a non-submitter's withdrawal), so the UI shows the withdraw action
  // only when this is true.
  readonly is_self: boolean;
}

// Normalized response of POST .../sick-note. Direct submissions carry the
// recorded days; approval-gated submissions carry an empty array.
export interface SickNoteSubmitResult {
  readonly status_days: StatusDay[];
  readonly pending_request?: ExcusedRequest;
}

// Resolved per-tenant parent-portal feature toggles for a child.
export interface ChildFeatures {
  // Zustand, keine Fähigkeit: Das Kind ist nicht mehr in Betreuung (#2487).
  // Ist es true, sind alle Schreib-Flags unten false — das Portal zeigt dann
  // ein Nur-Lesen-Profil mit einem Satz dazu statt Knöpfe, die alle gleich
  // scheitern würden.
  readonly care_ended?: boolean;
  readonly sick_note_enabled: boolean;
  // Whether a Krankmeldung stays pending until the OGS confirms it.
  readonly sick_requires_approval?: boolean;
  readonly notes_enabled: boolean;
  // Whether an "excused" absence submission must be confirmed by the OGS before
  // it takes effect (operations.parent_excused_requires_approval). When true the
  // child stays "expected" until an office/admin approves the request.
  readonly excused_requires_approval?: boolean;
  // Whether the guardian may submit structured change-requests (care schedule /
  // master data). Separate from notes_enabled (chat) so the UI hides the request
  // actions for a chat-only guardian instead of dead-ending on a backend 403.
  readonly request_submit_enabled: boolean;
  readonly pickup_change_enabled: boolean;
  readonly pickup_manage_allowed?: boolean;
  readonly guardian_contact_manage_allowed: boolean;
  readonly related_accounts_invite_enabled: boolean;
  readonly related_accounts_remove_enabled: boolean;
  readonly master_data_edit_enabled: boolean;
  readonly master_data_contact_edit_enabled: boolean;
  readonly master_data_request_enabled: boolean;
  readonly meal_plan_enabled: boolean;
  // STATE, not a capability: the child has a pending change request (master data
  // or care schedule) awaiting an OGS decision. Lets the overview badge the
  // Stammdaten entry without fetching the full request payloads.
  readonly has_open_change_request: boolean;
  // Whether the school broadcasts parent announcements
  // (operations.parent_news_enabled). When every linked school has it off the
  // Neuigkeiten feed is empty, so the parents-portal nav/panel entries gate on
  // this to avoid dead-ending on an empty page.
  readonly parent_news_enabled: boolean;
}

// One dish of the read-only meal plan (Essensplan) for a child's school.
// Mirrors api/parent.MealPlanEntryResponse. A day can carry several dishes.
export interface MealPlanEntry {
  readonly date: string; // YYYY-MM-DD
  readonly position: number;
  readonly dish: string;
  readonly note?: string | null;
}

// One day's pickup override plus OGS-owned arrival state for display.
// Times are "HH:MM" wall-clock strings; a missing leg has no override that day.
// `source` is "guardian" for parent-set entries, "staff" for ones the team set.
export interface CareException {
  readonly date: string;
  readonly pickup_time?: string;
  readonly arrival_time?: string;
  readonly reason?: string;
  readonly source: string;
  readonly pickup_source: string;
  readonly updated_at: string;
  // True when a pickup-exception row exists for the day but carries no time —
  // staff's "not coming today" absence marker (StudentPickupException.IsAbsent).
  // Distinct from a missing pickup_time meaning "no pickup override this day":
  // the tile resolves this to an absence, not the base-plan pickup. Absent (and
  // thus undefined) for ordinary rows.
  readonly pickup_absent?: boolean;
  // The arrival-leg counterpart: an arrival-exception row with no expected time
  // ("not coming today", StudentArrivalException.IsAbsent). Like pickup_absent it
  // creates no status day, so today_absent misses it; either leg being absent
  // resolves the tile to an absence rather than the base-plan pickup. Undefined
  // for ordinary rows.
  readonly arrival_absent?: boolean;
}

export interface PickupChangeRequest {
  readonly id: string;
  readonly date: string;
  readonly pickup_time: string;
  readonly previous_pickup_time?: string;
  readonly reason: string;
  readonly status: "pending" | "approved" | "rejected" | "withdrawn";
  readonly decision_reason?: string;
  readonly created_at: string;
  readonly reviewed_at?: string;
}

// A guardian linked to the child, with portal-access status.
export interface RelatedAccount {
  readonly guardian_profile_id: string;
  readonly first_name: string;
  readonly last_name: string;
  readonly email?: string;
  readonly relationship_type: string;
  // Per-child role preset on the link; used to hide the grant-access action
  // for school-managed social-worker contacts (#2172).
  readonly guardian_role?: string;
  readonly is_primary: boolean;
  readonly status: "active" | "pending" | "no_account" | "active_no_access";
  // Marks the requesting parent's own row. Self-removal is rejected by the
  // backend, so the panel hides the remove action for it.
  readonly is_self: boolean;
}

// One phone number of a guardian. Mirrors api/parent.guardianPhoneResponse.
interface GuardianPhone {
  readonly phone_number: string;
  readonly phone_type: string;
  readonly label?: string;
  readonly is_primary: boolean;
}

// A guardian of the child with contact + pickup detail and the caller's
// per-guardian edit capabilities. Mirrors api/parent.childGuardianResponse.
export interface ChildGuardian {
  readonly guardian_profile_id: string;
  readonly student_guardian_id: string;
  readonly first_name: string;
  readonly last_name: string;
  readonly email?: string;
  readonly phones: GuardianPhone[];
  readonly address_street?: string;
  readonly address_city?: string;
  readonly address_postal_code?: string;
  readonly relationship_type: string;
  readonly is_primary: boolean;
  readonly is_emergency_contact: boolean;
  readonly can_pickup: boolean;
  readonly pickup_notes?: string;
  readonly has_account: boolean;
  readonly is_self: boolean;
  readonly can_edit_contact: boolean;
  readonly can_manage_pickup: boolean;
  readonly contact_locked_own_account: boolean;
  readonly contact_locked_shared: boolean;
  readonly contact_locked_social_worker: boolean;
  readonly contact_locked_full_guardian: boolean;
}

// Payload for a guardian contact edit. Profile fields + phone list are
// replaced wholesale. Mirrors api/parent.updateGuardianContactRequest.
export interface GuardianContactPayload {
  readonly first_name: string;
  readonly last_name: string;
  readonly email?: string | null;
  readonly address_street?: string | null;
  readonly address_city?: string | null;
  readonly address_postal_code?: string | null;
  readonly phones: {
    readonly phone_number: string;
    readonly phone_type: string;
    readonly label?: string | null;
    readonly is_primary: boolean;
  }[];
}

// Creates a contact without app access. App access is handled independently by
// the related-account invitation flow.
export interface CreateGuardianContactPayload extends GuardianContactPayload {
  readonly relationship_type: "parent" | "guardian" | "relative" | "other";
  readonly can_pickup: boolean;
  readonly is_emergency_contact: boolean;
  readonly pickup_notes?: string | null;
}

// Payload for a per-child pickup/relationship edit. Every field optional; an
// omitted field is left unchanged. Mirrors api/parent.updateGuardianRelationshipRequest.
export interface GuardianRelationshipPayload {
  readonly can_pickup?: boolean;
  readonly is_emergency_contact?: boolean;
  readonly pickup_notes?: string | null;
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

async function patchJson<T>(url: string, body: unknown): Promise<T> {
  const response = await fetch(url, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!response.ok) await throwResponseError(url, response);
  return unwrapEnvelope((await response.json()) as ApiEnvelope<T>);
}

async function putJson<T>(url: string, body: unknown): Promise<T> {
  const response = await fetch(url, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
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

/**
 * Fetches the enrollment phases the calling parent's account may apply to,
 * across every school (linked or not). The backend pre-filters by eligibility
 * and pre-sorts (already-linked schools first, then by school name and service
 * start), so the picker renders the list as-is. Powers the "Neue Anmeldung"
 * picker at /parents/enroll.
 */
export async function listEnrollableSchools(): Promise<EnrollablePhase[]> {
  return getJson<EnrollablePhase[]>("/api/parent/me/enrollable-schools");
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
    throw await readEnrollmentError(
      response,
      "Anmeldung konnte nicht übermittelt werden",
      logger,
      "parent_submit_failed",
    );
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
 * Reports the child absent for one or more dates (YYYY-MM-DD) with the chosen
 * status: "sick" (Krankmeldung, flips the live sick flag when effective) or
 * "excused" (Termin/Abwesenheit, no live flag). Direct submissions return the
 * recorded days; approval-gated submissions return an empty status-day array
 * and are discovered by refetching the absence-request list. The backend
 * verifies the caller is a guardian of the child and that the school has the
 * feature enabled. A trimmed reason is required for both absence types; a
 * disabled school surfaces as a thrown error.
 */
export async function submitSickNote(
  studentId: string,
  dates: string[],
  reason: string,
  status: StudentStatusKind = "sick",
): Promise<SickNoteSubmitResult> {
  const res = await postJson<SickNoteSubmitResult | StatusDay[]>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/sick-note`,
    { dates, reason, status },
  );
  // Current backends return a bare array. Keep accepting the former envelope so
  // a rolling deployment still presents one stable shape to callers.
  return Array.isArray(res)
    ? { status_days: res, pending_request: undefined }
    : res;
}

/**
 * Lists the child's absence requests that went through an OGS approval gate,
 * newest first. Powers the pending and recently decided summary lines.
 */
export async function listExcusedRequests(
  studentId: string,
): Promise<ExcusedRequest[]> {
  return getJson<ExcusedRequest[]>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/excused-requests`,
  );
}

/**
 * Withdraws the guardian's own still-pending sick or excused absence request.
 * Returns the updated request (now `withdrawn`). The function retains its
 * legacy excused-only name. The backend rejects a withdraw once the OGS has
 * decided the request.
 */
export async function withdrawExcusedRequest(
  studentId: string,
  requestId: string,
): Promise<ExcusedRequest> {
  return deleteJson<ExcusedRequest>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/excused-requests/${encodeURIComponent(requestId)}`,
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
 * Der Tagesstatus eines Kindes (#2252). Zweistufig: `at_ogs` beantwortet die
 * eine Frage ("Ist mein Kind in der OGS?"), `state` erklaert sie.
 *
 * `at_ogs` ist `null`, wenn die Frage nicht belastbar zu beantworten ist. Das
 * Frontend leitet Ebene 1 NIE aus `state` ab; eine Ja/Nein-Aussage ohne Beleg
 * waere schlimmer als zu schweigen.
 */
type ChildTodayState =
  | "present"
  | "left"
  | "expected"
  | "not_arrived"
  | "absent"
  | "no_care"
  | "unknown";

export interface ChildToday {
  readonly at_ogs: boolean | null;
  readonly state: ChildTodayState;
  /** "HH:MM", nur bei present. */
  readonly since?: string;
  /** "HH:MM", nur bei left. */
  readonly until?: string;
  /** "HH:MM", nur bei expected und not_arrived. */
  readonly expected_from?: string;
  /** "HH:MM", nur wenn das Kind gerade in der OGS ist. */
  readonly pickup_time?: string;
}

/** Was angezeigt wird, solange oder falls der Endpunkt nichts liefert. */
export const UNKNOWN_CHILD_TODAY: ChildToday = {
  at_ogs: null,
  state: "unknown",
};

/**
 * Holt den Tagesstatus des Kindes.
 *
 * Faellt auf `unknown` zurueck, statt zu werfen: der Endpunkt kann fehlen
 * (404, waehrend das Backend nachzieht) oder der Zugriff auf das Kind ist
 * entzogen (403). Beides ist "wir wissen es nicht", kein Fehlerzustand, den ein
 * Elternteil sehen muesste. 401 laesst throwResponseError weiterhin zur
 * Anmeldung umleiten.
 *
 * Genau diese zwei Codes, nicht "ab 403": ein 500er oder ein ausgefallener
 * Proxy ist ein echter Fehler und darf nicht als gueltiger Unbekannt-Zustand
 * durchgehen. Alle Aufrufer fangen den Wurf ab und zeigen weiterhin
 * UNKNOWN_CHILD_TODAY — nur bleibt der Ausfall jetzt als Fehler sichtbar.
 */
export async function getChildToday(studentId: string): Promise<ChildToday> {
  try {
    return await getJson<ChildToday>(
      `/api/parent/me/children/${encodeURIComponent(studentId)}/today`,
    );
  } catch (err) {
    if (
      err instanceof ParentApiError &&
      (err.status === 403 || err.status === 404)
    ) {
      logger.warn("parent_child_today_unavailable", {
        status: err.status,
        student_id: studentId,
      });
      return UNKNOWN_CHILD_TODAY;
    }
    throw err;
  }
}

/**
 * Fetches the Monday-Friday meal plan for the child's school for the week
 * containing weekStart (YYYY-MM-DD). Returns 403 (meal_plan_disabled) when the
 * school does not run a meal plan; callers gate on meal_plan_enabled first.
 */
export async function getChildMealPlan(
  studentId: string,
  weekStart: string,
): Promise<MealPlanEntry[]> {
  return getJson<MealPlanEntry[]>(
    `/api/parent/me/children/${encodeURIComponent(
      studentId,
    )}/meal-plan?week_start=${encodeURIComponent(weekStart)}`,
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

// --- Parent <-> OGS messaging (chat model) ---
//
// One continuous conversation per child between the guardian and the OGS (no
// subject). The guardian always talks to "the OGS [Schulname]", never an
// individual staff member — the backend masks staff names accordingly.

// One message in a conversation. `sender_kind` is "guardian" for the parent's
// own messages, "staff" for replies from the OGS. For staff messages
// `sender_name` is the "OGS [Schulname]" label.
// The wire message shape is shared with the staff client; see ChatMessage.
export type ParentMessage = ChatMessage;

// One row on the messages landing page: a child's conversation, with the
// guardian's unread (staff-sent) count and last-activity metadata.
export interface ThreadSummary {
  readonly thread_id: string;
  readonly student_id: string;
  readonly student_name: string;
  readonly school_name: string;
  readonly counterpart_name: string; // "OGS [Schulname]"
  readonly last_message_at?: string; // ISO timestamp
  readonly last_sender_kind?: "guardian" | "staff";
  readonly last_message_body?: string;
  // Structured fields of the last message so the localized portal previews a
  // request title / decision / withdrawal from fields instead of the German
  // last_message_body (see parentThreadPreviewI18nDescriptor). Empty for plain
  // messages, where last_message_body is the language-neutral written text.
  readonly last_message_kind?: "message" | "event" | "request";
  readonly last_event_type?: string;
  readonly last_request_type?: string;
  readonly last_request_status?: string;
  readonly last_message_payload?: Record<string, unknown>;
  readonly last_message_read_by_staff: boolean;
  readonly unread: number;
}

// One parent-news feed entry (#1669). read/acknowledged are THIS guardian's
// state; requires_acknowledgement tells the app whether to offer the
// "gelesen und bestätigt" action.
export interface ParentAnnouncement {
  readonly id: string;
  readonly title: string;
  readonly body: string;
  readonly priority: "info" | "important";
  readonly link_url?: string;
  readonly requires_acknowledgement: boolean;
  readonly school_name: string;
  readonly published_at?: string; // ISO timestamp
  readonly expires_at?: string; // ISO timestamp
  readonly read: boolean;
  readonly acknowledged: boolean;

  // Umfrage fields (#1371). "none" is a plain Mitteilung; a choice type comes
  // with the answer options and the guardian's own children this poll reaches
  // (one answer row per child).
  readonly response_type: "none" | "single_choice" | "multi_choice";
  readonly response_deadline?: string; // ISO timestamp
  readonly options?: readonly ParentAnnouncementOption[];
  readonly children?: readonly ParentAnnouncementPollChild[];
}

interface ParentAnnouncementOption {
  readonly id: string;
  readonly label: string;
}

/** One of the guardian's children, with the options selected for that child. */
export interface ParentAnnouncementPollChild {
  readonly student_id: string;
  readonly first_name: string;
  readonly last_name: string;
  readonly selected_options: readonly string[];
}

// A child's full conversation (messages oldest-first). `thread_id` is empty
// when the guardian has not written about this child yet.
export interface ThreadView {
  readonly thread_id: string;
  readonly student_id: string;
  readonly student_name: string;
  readonly school_name: string;
  readonly counterpart_name: string; // "OGS [Schulname]"
  readonly messages: ParentMessage[];
}

/** Lists the guardian's conversations (one per child written about). */
export async function listMessageThreads(): Promise<ThreadSummary[]> {
  return getJson<ThreadSummary[]>("/api/parent/me/messages");
}

/**
 * Lists the guardian's conversation(s) about ONE child (at most one per the chat
 * model). The child detail page uses this instead of fetching the whole
 * cross-tenant inbox and filtering client-side. Reading it does NOT mark the
 * thread read.
 */
export async function listChildThreads(
  studentId: string,
): Promise<ThreadSummary[]> {
  return getJson<ThreadSummary[]>(
    `/api/parent/me/messages/children/${encodeURIComponent(studentId)}/threads`,
  );
}

/**
 * Total number of conversations with unread staff-side activity, across all the
 * guardian's children's schools — the sidebar badge. A light COUNT endpoint, so
 * the badge does not fetch every thread's full projection just to sum unreads.
 */
export async function fetchMessagesUnreadCount(): Promise<number> {
  const result = await getJson<{ unread_count: number }>(
    "/api/parent/me/messages/unread-count",
  );
  return result.unread_count ?? 0;
}

/**
 * The guardian's parent-news feed across all their (news-enabled) children's
 * schools, newest-published first. Visibility + audience are enforced
 * server-side from the JWT account.
 */
export async function listAnnouncements(): Promise<ParentAnnouncement[]> {
  return getJson<ParentAnnouncement[]>("/api/parent/me/news");
}

/**
 * Number of parent letters that still need attention. This includes unread
 * letters, pending read confirmations, and unanswered open surveys.
 */
export async function fetchAnnouncementsUnreadCount(): Promise<number> {
  const result = await getJson<{ unread_count: number }>(
    "/api/parent/me/news/unread-count",
  );
  return result.unread_count ?? 0;
}

/**
 * Marks an announcement read for this guardian (idempotent). `publishedAt` is
 * the version the client loaded; the backend rejects the request (409) if the
 * announcement has since been corrected/republished, so a stale tab cannot
 * record a read for wording the guardian never saw.
 */
export async function markAnnouncementRead(
  announcementId: string,
  publishedAt: string,
): Promise<void> {
  await postJson<{ read: boolean }>(
    `/api/parent/me/news/${encodeURIComponent(announcementId)}/read`,
    { published_at: publishedAt },
  );
}

/**
 * Records an explicit "gelesen und bestätigt" for an announcement. `publishedAt`
 * is verified against the live announcement (see markAnnouncementRead) so a
 * confirmation is never counted against since-corrected wording.
 */
export async function acknowledgeAnnouncement(
  announcementId: string,
  publishedAt: string,
): Promise<void> {
  await postJson<{ acknowledged: boolean }>(
    `/api/parent/me/news/${encodeURIComponent(announcementId)}/acknowledge`,
    { published_at: publishedAt },
  );
}

/**
 * Records the guardian's answer to an Umfrage for ONE child (#1371).
 * `optionIds` replaces whatever was selected for that child; an empty array
 * withdraws the answer. Same `publishedAt` version guard as the read/ack calls,
 * plus the server-side answer deadline.
 */
export async function respondToAnnouncement(
  announcementId: string,
  studentId: string,
  optionIds: readonly string[],
  publishedAt: string,
): Promise<void> {
  await postJson<{ answered: boolean }>(
    `/api/parent/me/news/${encodeURIComponent(announcementId)}/respond`,
    {
      student_id: studentId,
      option_ids: optionIds,
      published_at: publishedAt,
    },
  );
}

/**
 * Fetches the guardian's conversation about one child (oldest-first), creating
 * nothing — an empty conversation comes back with an empty `thread_id`. Reading
 * marks it read server-side.
 */
export async function getChildConversation(
  studentId: string,
): Promise<ThreadView> {
  return getJson<ThreadView>(
    `/api/parent/me/messages/children/${encodeURIComponent(studentId)}`,
  );
}

/**
 * Appends a guardian message to the child's conversation (created on the first
 * message) and returns the full updated conversation.
 */
export async function postChildMessage(
  studentId: string,
  body: string,
): Promise<ThreadView> {
  return postJson<ThreadView>(
    `/api/parent/me/messages/children/${encodeURIComponent(studentId)}`,
    { body },
  );
}

/**
 * Sets a one-day pickup override for the child. Arrival times remain under OGS
 * control. The backend verifies guardianship, the feature gate, and refuses to
 * overwrite a staff-set pickup exception.
 */
export async function submitCareException(
  studentId: string,
  params: {
    date: string;
    pickupTime: string;
    reason: string;
  },
): Promise<PickupChangeRequest> {
  return postJson<PickupChangeRequest>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/care-exception`,
    {
      date: params.date,
      pickup_time: params.pickupTime,
      reason: params.reason,
    },
  );
}

export async function listPickupChangeRequests(
  studentId: string,
): Promise<PickupChangeRequest[]> {
  return getJson<PickupChangeRequest[]>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/pickup-change-requests`,
  );
}

export async function withdrawPickupChangeRequest(
  studentId: string,
  requestId: string,
): Promise<PickupChangeRequest> {
  return deleteJson<PickupChangeRequest>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/pickup-change-requests/${encodeURIComponent(requestId)}`,
  );
}

// --- Related accounts (who has parents-app access to the child) ---

/** Lists guardians linked to the child, with portal-access status. */
export async function listRelatedAccounts(
  studentId: string,
): Promise<RelatedAccount[]> {
  return getJson<RelatedAccount[]>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/related-accounts`,
  );
}

// --- Guardian contact + pickup info (#1667) ---

/** Lists the child's guardians with contact + pickup detail and edit caps. */
export async function listChildGuardians(
  studentId: string,
): Promise<ChildGuardian[]> {
  return getJson<ChildGuardian[]>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/guardians`,
  );
}

/** Adds an accountless contact to the child. */
export async function createGuardianContact(
  studentId: string,
  payload: CreateGuardianContactPayload,
): Promise<ChildGuardian> {
  return postJson<ChildGuardian>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/guardians`,
    payload,
  );
}

/** Updates a contact-only guardian's contact data (or the caller's own). */
export async function updateGuardianContact(
  studentId: string,
  guardianProfileId: string,
  payload: GuardianContactPayload,
): Promise<ChildGuardian> {
  return putJson<ChildGuardian>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/guardians/${encodeURIComponent(guardianProfileId)}/contact`,
    payload,
  );
}

/** Updates the per-child pickup/relationship fields of a guardian. */
export async function updateGuardianRelationship(
  studentId: string,
  guardianProfileId: string,
  payload: GuardianRelationshipPayload,
): Promise<ChildGuardian> {
  return putJson<ChildGuardian>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/guardians/${encodeURIComponent(guardianProfileId)}/pickup`,
    payload,
  );
}

// Outcome echoed by the invite resolve.
export interface InviteRelatedAccountResult {
  readonly outcome: string;
  readonly guardian_profile_id: string;
  // Current guardian role for the existing_contact_restricted outcome, so the
  // UI can name it in the upgrade confirmation.
  readonly existing_role?: string;
}

/** Invites a further guardian to the child by email. */
export async function inviteRelatedAccount(
  studentId: string,
  email: string,
  options?: {
    firstName?: string;
    lastName?: string;
    confirmRoleUpgrade?: boolean;
  },
): Promise<InviteRelatedAccountResult> {
  return postJson<InviteRelatedAccountResult>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/related-accounts`,
    {
      email,
      first_name: options?.firstName ?? "",
      last_name: options?.lastName ?? "",
      // Only sent when confirming an upgrade; a plain invite keeps the
      // historical three-field body.
      ...(options?.confirmRoleUpgrade ? { confirm_role_upgrade: true } : {}),
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

// --- Care schedule (standard weekly plan + permanent change requests, #1803) ---

// One weekday of the child's standard weekly care plan. Mirrors
// api/parent.CareScheduleWeekdayResponse. `weekday` is ISO (Monday=1 .. Friday=5).
// `arrival`/`pickup` are "HH:MM" wall-clock strings, absent/empty when unset.
// `modes` are the allowed departure-mode keys for the day (e.g. ["bus","pickup"]).
interface CareScheduleWeekday {
  readonly weekday: number;
  readonly status: "scheduled" | "not_scheduled" | "unknown";
  readonly arrival?: string;
  readonly pickup?: string;
  readonly modes: string[];
}

// The guardian's still-open permanent change request for the care schedule.
// Mirrors api/parent.PendingCareRequestResponse. `diff` reuses the shared
// RequestDiffEntry wire shape (label/old/new + structured discriminators), so
// the localized parents portal can render each row in the guardian's language.
// `submitted_by_self` is true only for the calling guardian's own request —
// withdraw is offered only then.
interface PendingCareRequest {
  readonly id: string;
  readonly created_at: string; // ISO timestamp
  readonly diff: RequestDiffEntry[];
  readonly submitted_by_self: boolean;
}

// The read view of a child's care schedule. Mirrors api/parent.CareScheduleResponse.
export interface ChildCareSchedule {
  readonly weekdays: CareScheduleWeekday[];
  readonly pending_request?: PendingCareRequest;
  readonly can_request: boolean;
  readonly request_capabilities: {
    readonly arrival: boolean;
    readonly pickup: boolean;
    readonly departure_mode: boolean;
  };
  // True when the child has any active scheduled absence today (sick, excused,
  // or class trip — any source). The parent-safe absence signal for the "Heute
  // → Abholung" tile: the windowed sick-day list (listSickDays) hides
  // staff-created excused/class-trip days, so the tile relies on this boolean to
  // avoid showing a pickup time for a child the school has recorded as off.
  readonly today_absent: boolean;
  // The Berlin calendar day (YYYY-MM-DD) the backend resolved today_absent
  // against — its "today" at request-handling time. The "Heute" tile binds its
  // cached absence signal to THIS date rather than the browser's request-start
  // day, so a response that crosses Berlin midnight can't stamp the new day's
  // today_absent onto yesterday's pickup tile (#1725). Only the GET read view
  // populates it; absent on the request-write responses.
  readonly today_date?: string;
}

/**
 * Fetches the child's standard weekly care plan (Mon-Fr), the guardian's own
 * still-open change request (if any), and whether a new request may be
 * submitted. Powers the care-schedule section on the Stammdaten page.
 */
export async function getChildCareSchedule(
  studentId: string,
): Promise<ChildCareSchedule> {
  return getJson<ChildCareSchedule>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/care-schedule`,
  );
}

// --- Booked care offerings (#1665, #2303) ---

/** One booked care offering of the child's current care period. */
interface CareOfferingItem {
  readonly id: string;
  readonly name: string;
  readonly description?: string;
  /** Booked ISO weekdays (1=Mon..7=Sun); empty when the offering has no per-day choice. */
  readonly weekdays: number[];
  readonly price_cents?: number;
  readonly includes_lunch: boolean;
  readonly includes_holiday_care: boolean;
  /** First day of a scheduled future booking (YYYY-MM-DD). */
  readonly valid_from?: string;
  /** Last day of a superseded booking (YYYY-MM-DD). */
  readonly valid_until?: string;
  /** True while a booked offering has not started yet. */
  readonly starts_later?: boolean;
}

/** One "current -> requested" line of a pending offering change. */
interface OfferingDiffLine {
  readonly label: string;
  readonly old_state: "not_booked" | "booked";
  /** Canonical day keys; empty for an all-day booking. */
  readonly old_days: string[];
  readonly new_state: "removed" | "booked";
  /** Canonical day keys; empty for an all-day booking. */
  readonly new_days: string[];
  /** Share of new_days added by a co-booking rule or required lunch (#2365). */
  readonly new_automatic_days?: string[];
  /** Rule-derived part of new_automatic_days, excluding required-lunch days. */
  readonly new_rule_days?: string[];
  /** Names of the selected offerings whose rule added new_rule_days. */
  readonly auto_trigger_names?: string[];
}

/** The child's open offering change request. */
interface PendingOfferingChange {
  readonly id: string;
  readonly created_at: string;
  /** Date the switch would take effect (YYYY-MM-DD). */
  readonly effective_from: string;
  readonly note?: string;
  readonly diff: OfferingDiffLine[];
  readonly submitted_by_self: boolean;
}

/** One offering of a decided request. */
interface OfferingRequestedItem {
  readonly id: string;
  readonly name: string;
  /** ISO weekdays (1=Mon..7=Sun); empty means every care day. */
  readonly weekdays: number[];
}

/** A decided change request: the outcome plus a rejection's reason. */
interface OfferingDecision {
  readonly id: string;
  readonly status: "approved" | "rejected";
  readonly decided_at: string;
  /** Date the switch took (or would have taken) effect (YYYY-MM-DD). */
  readonly effective_from: string;
  readonly reason?: string;
  /** What the family asked for, so the decision stays readable on its own. */
  readonly requested: OfferingRequestedItem[];
  /** The frozen diff the decision was made on; absent for older decisions. */
  readonly applied?: OfferingDiffLine[];
  /** Rule-added offerings the school excluded for this one request (#2370). */
  readonly overridden_names?: string[];
}

/** Why the change button is unavailable. Stable identifiers from the backend. */
export type OfferingChangesDisabledReason =
  | "no_enrollment"
  | "no_permission"
  | "school_disabled"
  | "period_over"
  | "no_time_remaining";

/** Parent-facing view of what the child is booked into. */
export interface ChildCareOfferings {
  readonly period_name?: string;
  readonly period_start?: string;
  readonly period_end?: string;
  readonly offerings: CareOfferingItem[];
  readonly can_request: boolean;
  readonly pending_request?: PendingOfferingChange;
  /** Earliest date a new request may take effect (YYYY-MM-DD). */
  readonly earliest_effective_from?: string;
  /** Most recent decided request, dropped once it ages out of the window. */
  readonly last_decision?: OfferingDecision;
  readonly changes_disabled_reason?: OfferingChangesDisabledReason;
}

/** One selectable offering in the change-request modal. */
export interface OfferingCatalogItem {
  readonly id: string;
  readonly name: string;
  readonly description?: string;
  /** "fixed" = the school sets the days; "parent_choice" = the family picks. */
  readonly days_of_week_mode: string;
  readonly available_days: string[];
  readonly selection_group?: string;
  readonly selection_rule: string;
  readonly is_required: boolean;
  readonly price_cents?: number;
  readonly includes_lunch: boolean;
  readonly includes_holiday_care: boolean;
  readonly selected: boolean;
  readonly selected_days: string[];
  /** True when the current booking is derived from another offering. */
  readonly automatic?: boolean;
  /** False for a booking retained after the school deactivated the offering. */
  readonly is_active?: boolean;
  readonly capacity?: number;
  readonly free_slots?: number;
}

/** The selectable catalog plus the allowed effective-date window. */
export interface OfferingCatalog {
  readonly phase_name: string;
  readonly selection_mode: string;
  readonly earliest_effective_from: string;
  readonly latest_effective_from: string;
  readonly items: OfferingCatalogItem[];
}

/** One desired offering in a submission. */
export interface OfferingChangeSelectionInput {
  readonly offering_id: string;
  readonly selected_days: string[];
}

/** Returns the care offerings the child is booked into. */
export async function getChildCareOfferings(
  studentId: string,
): Promise<ChildCareOfferings> {
  return getJson<ChildCareOfferings>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/care-offerings`,
  );
}

/** Returns the offerings the guardian may pick from, prefilled. */
export async function getChildOfferingCatalog(
  studentId: string,
  effectiveFrom?: string,
): Promise<OfferingCatalog> {
  const params = effectiveFrom
    ? `?effective_from=${encodeURIComponent(effectiveFrom)}`
    : "";
  return getJson<OfferingCatalog>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/care-offerings/catalog${params}`,
  );
}

/**
 * Submits a post-enrollment offering change. The selection is the COMPLETE
 * desired booking, not a delta: staff decide one coherent booking, and the
 * backend re-validates it against capacity and the phase rules on approval.
 */
export async function submitOfferingChangeRequest(
  studentId: string,
  input: {
    offerings: OfferingChangeSelectionInput[];
    effective_from: string;
    note?: string;
  },
): Promise<ChildCareOfferings> {
  return postJson<ChildCareOfferings>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/care-offerings/requests`,
    input,
  );
}

/** Withdraws the guardian's own still-open offering change request. */
export async function withdrawOfferingChangeRequest(
  studentId: string,
  requestId: string,
): Promise<ChildCareOfferings> {
  return postJson<ChildCareOfferings>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/care-offerings/requests/${encodeURIComponent(requestId)}/withdraw`,
    {},
  );
}

// --- Stammdaten (master data view + change flow) ---

// One change-request row in the parent view. Mirrors
// api/parent.MasterDataChangeResponse. old/new values are JSON (string for the
// fields shipped today; object for departure modes).
export interface MasterDataChange {
  readonly id: string;
  readonly target: string;
  readonly field_key: string;
  readonly old_value?: unknown;
  readonly new_value: unknown;
  readonly status: "auto_applied" | "pending" | "approved" | "rejected";
  readonly created_at: string;
}

// The structured Stammdaten view. Mirrors api/parent.MasterDataResponse.
export interface ChildMasterData {
  readonly student_id: string;
  readonly first_name: string;
  readonly last_name: string;
  readonly birthday?: string;
  readonly school_class: string;
  readonly status: string;
  readonly enrolled_from?: string;
  readonly enrolled_until?: string;
  readonly health_info?: string;
  readonly guardian_profile_id: string;
  readonly email?: string;
  readonly address_street?: string;
  readonly address_city?: string;
  readonly address_postal_code?: string;
  readonly preferred_contact_method: string;
  readonly language_preference: string;
  readonly primary_phone?: string;
  readonly departure_days?: Record<string, string>;
  readonly allowed_departure_modes?: Record<string, string[]>;
  readonly pending_changes: MasterDataChange[];
}

/** Fetches the child's structured Stammdaten view (guardian's own data included). */
export async function getChildMasterData(
  studentId: string,
): Promise<ChildMasterData> {
  return getJson<ChildMasterData>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/master-data`,
  );
}

/**
 * Applies a Track A direct edit to a single field and returns the refreshed
 * view. `value` is JSON (a string for every Track A field today).
 */
export async function updateMasterDataField(
  studentId: string,
  target: string,
  field: string,
  value: unknown,
): Promise<ChildMasterData> {
  return patchJson<ChildMasterData>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/master-data/${encodeURIComponent(target)}/${encodeURIComponent(field)}`,
    { value },
  );
}

/** One proposed Track B change. value is JSON (string or departure-modes object). */
export interface MasterDataChangeInput {
  readonly target: string;
  readonly field_key: string;
  readonly value: unknown;
}

/** Submits Track B change requests (name, birthday, class, permanent Gehzeit) for approval. */
export async function submitMasterDataRequest(
  studentId: string,
  changes: MasterDataChangeInput[],
): Promise<MasterDataChange[]> {
  return postJson<MasterDataChange[]>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/master-data/requests`,
    { changes },
  );
}

/** Removes another account's access to the child (not the primary guardian). */
export async function removeRelatedAccount(
  studentId: string,
  guardianProfileId: string,
): Promise<void> {
  const response = await fetch(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/related-accounts/${encodeURIComponent(guardianProfileId)}`,
    { method: "DELETE" },
  );
  if (!response.ok) {
    let message = `Request failed (${response.status})`;
    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // Body was not JSON, keep the generic message.
    }
    if (response.status === 401 && typeof window !== "undefined") {
      window.location.assign("/parents/login");
    }
    logger.error("parent_api_request_failed", {
      url: "remove_related_account",
      status: response.status,
      message,
    });
    throw new Error(message);
  }
}
