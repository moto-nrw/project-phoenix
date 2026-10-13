import type { DepartureMode } from "~/lib/student-helpers";
import { ApiError, apiErrorFromBody, transportFetch } from "./api-error";
/**
 * Staff client for the parent care-schedule change-request review queue
 * (#1803). Calls the Next.js proxy routes under
 * /api/students/care-schedule-change-requests which forward (with the tenant
 * session token) to the backend.
 */

import { createLogger } from "~/lib/logger";
import type { RequestDiffEntry } from "~/lib/messaging-status";

const logger = createLogger({ component: "CareRequestReviewAPI" });

type CareRequestStatus =
  "pending" | "approved" | "rejected" | "withdrawn" | "done" | "care_ended";

// One care-schedule change request in the staff queue. Mirrors
// api/students.CareRequestResponse.
export interface StaffCareRequest {
  readonly id: string;
  readonly student_id: string;
  readonly first_name: string;
  readonly last_name: string;
  readonly status: CareRequestStatus;
  readonly request_kind: "weekly_schedule" | "pickup_change";
  readonly diff: readonly RequestDiffEntry[];
  readonly request_reason?: string;
  readonly decision_reason?: string;
  readonly created_at: string;
  readonly reviewed_at?: string;
  readonly affected_blocks: readonly AffectedCareBlock[];
  readonly impact_available: boolean;
  readonly impact_token: string;
}

interface AffectedCareBlock {
  readonly id: string;
  readonly title: string;
  readonly start_time: string;
  readonly end_time: string;
}

interface Envelope<T> {
  readonly data?: T;
}

function unwrap<T>(json: Envelope<T>): T {
  if (json && typeof json === "object" && "data" in json) {
    return json.data as T;
  }
  return json as unknown as T;
}

/**
 * Turns a failed response into an ApiError. The backend's stable 409 `code`
 * (e.g. "students.guardian_access_revoked", "students.change_request_not_pending")
 * lets the review UI name the concrete recovery action, and `status` tells
 * "gone" from "no access"; `detail` stays diagnostics for the log.
 */
async function readError(
  response: Response,
  fallback: string,
): Promise<ApiError> {
  let payload: unknown;
  try {
    payload = await response.json();
  } catch {
    // not JSON
  }
  const error = apiErrorFromBody(fallback, response.status, payload);
  logger.error("care_request_review_request_failed", {
    status: response.status,
    message: error.message,
    ...(error.code ? { code: error.code } : {}),
  });
  return error;
}

/** Approves (applies the weekly plan) or rejects one care-schedule request. */
export async function decideCareScheduleChangeRequest(
  requestId: string,
  approve: boolean,
  reason: string | undefined,
  impactToken: string,
  expectedVersion?: string,
): Promise<StaffCareRequest> {
  const response = await transportFetch(
    `/api/students/care-schedule-change-requests/${encodeURIComponent(requestId)}/decide`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        approve,
        reason: reason ?? "",
        impact_token: impactToken,
        ...(expectedVersion ? { expected_version: expectedVersion } : {}),
      }),
    },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Entscheidung konnte nicht gespeichert werden",
    );
  }
  return unwrap((await response.json()) as Envelope<StaffCareRequest>);
}

/**
 * One decided care-schedule change request in the staff history. Mirrors
 * api/students.CareRequestHistoryResponse: `diff` replays the alt → neu
 * comparison frozen at decision time (#2430); rows decided before the
 * snapshot existed (and withdrawals) omit it, and the payload-derived
 * requested summary (each entry's old side empty) is the fallback.
 */
export interface StaffCareRequestHistoryEntry {
  readonly id: string;
  readonly student_id: string;
  readonly first_name: string;
  readonly last_name: string;
  readonly status: CareRequestStatus;
  readonly request_kind: "weekly_schedule" | "pickup_change";
  readonly requested: readonly RequestDiffEntry[];
  /** Frozen decision-time diff; absent without a snapshot. */
  readonly diff?: readonly RequestDiffEntry[];
  readonly decision_reason?: string;
  readonly created_at: string;
  readonly decided_at: string;
  /** Absent for withdrawn rows (no reviewer). */
  readonly decided_by_name?: string;
}

/**
 * One care-schedule request of ANY status, read from a message-thread pill
 * (#3135). Mirrors api/students.CareRequestDetailResponse: the history
 * projection plus the guardian's reason; a pending row carries no decision.
 */
export interface StaffCareRequestDetail {
  readonly id: string;
  readonly student_id: string;
  readonly first_name: string;
  readonly last_name: string;
  readonly status: CareRequestStatus;
  readonly request_kind: "weekly_schedule" | "pickup_change";
  readonly requested: readonly RequestDiffEntry[];
  /** Frozen decision-time diff; absent without a snapshot. */
  readonly diff?: readonly RequestDiffEntry[];
  readonly request_reason?: string;
  readonly decision_reason?: string;
  readonly created_at: string;
  /** Absent while the request is still pending. */
  readonly decided_at?: string;
  readonly decided_by_name?: string;
  /**
   * The stored ask of a pickup-change row: calendar day ("YYYY-MM-DD"),
   * requested time ("HH:MM") and the time recorded at submission, if any.
   * Absent for weekly-plan rows.
   */
  readonly pickup_change?: {
    readonly date: string;
    /** Absent when the request leaves the Gehzeit of the day (#3945). */
    readonly pickup_time?: string;
    readonly previous_pickup_time?: string;
    /** The Abholart asked for the day, absent when it stays (#3945). */
    readonly departure_mode?: DepartureMode;
    readonly previous_departure_modes?: readonly DepartureMode[];
  };
}

/**
 * Loads one care-schedule request by its row id, whatever its status. The
 * backend answers 404 for a removed or foreign row and 403 for a child the
 * reader may not review; both arrive as ApiError with `status`.
 */
export async function fetchCareScheduleChangeRequest(
  requestId: string,
): Promise<StaffCareRequestDetail> {
  const response = await transportFetch(
    `/api/students/care-schedule-change-requests/${encodeURIComponent(requestId)}`,
  );
  if (!response.ok) {
    throw await readError(response, "Anfrage konnte nicht geladen werden");
  }
  return unwrap((await response.json()) as Envelope<StaffCareRequestDetail>);
}
