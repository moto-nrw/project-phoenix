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

// Resolved per-tenant parent-portal feature toggles for a child.
export interface ChildFeatures {
  readonly sick_note_enabled: boolean;
  readonly notes_enabled: boolean;
  // Whether the guardian may submit structured change-requests (care schedule /
  // master data). Separate from notes_enabled (chat) so the UI hides the request
  // actions for a chat-only guardian instead of dead-ending on a backend 403.
  readonly request_submit_enabled: boolean;
  readonly pickup_change_enabled: boolean;
  readonly related_accounts_invite_enabled: boolean;
  readonly related_accounts_remove_enabled: boolean;
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

// A guardian linked to the child, with portal-access status.
export interface RelatedAccount {
  readonly guardian_profile_id: string;
  readonly first_name: string;
  readonly last_name: string;
  readonly email?: string;
  readonly relationship_type: string;
  readonly is_primary: boolean;
  readonly status: "active" | "pending" | "no_account";
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
 * Reports the child absent for one or more dates (YYYY-MM-DD) with the chosen
 * status: "sick" (Krankmeldung, flips the live sick flag) or "excused"
 * (Termin/Abwesenheit, no live flag). Returns the just-submitted days. The
 * backend verifies the caller is a guardian of the child and that the school
 * has the feature enabled; a disabled school surfaces as a thrown error.
 */
export async function submitSickNote(
  studentId: string,
  dates: string[],
  reason = "",
  status: StudentStatusKind = "sick",
): Promise<StatusDay[]> {
  return postJson<StatusDay[]>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/sick-note`,
    { dates, reason, status },
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
export type { RequestDiffEntry };

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
  readonly unread: number;
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
 * Submits a structured change-request (care schedule / master data) for the
 * child and returns the full updated conversation (the request appears as a
 * "request" timeline entry awaiting OGS confirmation).
 */
export async function createChildRequest(
  studentId: string,
  requestType: "care_schedule" | "student_master_data",
  payload: Record<string, unknown>,
): Promise<ThreadView> {
  return postJson<ThreadView>(
    `/api/parent/me/messages/children/${encodeURIComponent(studentId)}/requests`,
    { request_type: requestType, payload },
  );
}

/**
 * Withdraws a still-open change-request and returns the full updated
 * conversation (the request flips to "zurueckgezogen").
 */
export async function withdrawChildRequest(
  studentId: string,
  requestId: string,
): Promise<ThreadView> {
  return postJson<ThreadView>(
    `/api/parent/me/messages/children/${encodeURIComponent(studentId)}/requests/${encodeURIComponent(requestId)}/withdraw`,
    {},
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
}

/** Invites a further guardian to the child by email. */
export async function inviteRelatedAccount(
  studentId: string,
  email: string,
  options?: { firstName?: string; lastName?: string },
): Promise<InviteRelatedAccountResult> {
  return postJson<InviteRelatedAccountResult>(
    `/api/parent/me/children/${encodeURIComponent(studentId)}/related-accounts`,
    {
      email,
      first_name: options?.firstName ?? "",
      last_name: options?.lastName ?? "",
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
