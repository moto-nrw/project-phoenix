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
 * Error thrown by the care-request client. Carries the backend's stable 409
 * `code` (e.g. "messaging_disabled", "guardian_access_revoked",
 * "change_request_not_pending") so the review UI can name the concrete recovery
 * action instead of collapsing every failure into one generic message. The raw
 * `error` string stays the Error message for logging.
 */
export class CareRequestApiError extends Error {
  readonly code?: string;
  /** HTTP status of the failed response, so a reader can tell "weg" from "kein Zugriff". */
  readonly status?: number;
  constructor(message: string, code?: string, status?: number) {
    super(message);
    this.name = "CareRequestApiError";
    this.code = code;
    this.status = status;
  }
}

async function readError(
  response: Response,
  fallback: string,
): Promise<CareRequestApiError> {
  let message = fallback;
  let code: string | undefined;
  try {
    const body = (await response.json()) as { error?: string; code?: string };
    if (body.error) message = body.error;
    if (body.code) code = body.code;
  } catch {
    // not JSON
  }
  logger.error("care_request_review_request_failed", {
    status: response.status,
    message,
    ...(code ? { code } : {}),
  });
  return new CareRequestApiError(message, code, response.status);
}

/** Approves (applies the weekly plan) or rejects one care-schedule request. */
export async function decideCareScheduleChangeRequest(
  requestId: string,
  approve: boolean,
  reason: string | undefined,
  impactToken: string,
  expectedVersion?: string,
): Promise<StaffCareRequest> {
  const response = await fetch(
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
    readonly pickup_time: string;
    readonly previous_pickup_time?: string;
  };
}

/**
 * Loads one care-schedule request by its row id, whatever its status. The
 * backend answers 404 for a removed or foreign row and 403 for a child the
 * reader may not review; both arrive as CareRequestApiError with `status`.
 */
export async function fetchCareScheduleChangeRequest(
  requestId: string,
): Promise<StaffCareRequestDetail> {
  const response = await fetch(
    `/api/students/care-schedule-change-requests/${encodeURIComponent(requestId)}`,
  );
  if (!response.ok) {
    throw await readError(response, "Anfrage konnte nicht geladen werden");
  }
  return unwrap((await response.json()) as Envelope<StaffCareRequestDetail>);
}
