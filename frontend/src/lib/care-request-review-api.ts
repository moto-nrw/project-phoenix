/**
 * Staff client for the parent care-schedule change-request review queue
 * (#1803). Calls the Next.js proxy routes under
 * /api/students/care-schedule-change-requests which forward (with the tenant
 * session token) to the backend.
 */

import { createLogger } from "~/lib/logger";
import type { RequestDiffEntry } from "~/lib/messaging-status";

const logger = createLogger({ component: "CareRequestReviewAPI" });

type CareRequestStatus = "pending" | "approved" | "rejected" | "withdrawn";

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
  constructor(message: string, code?: string) {
    super(message);
    this.name = "CareRequestApiError";
    this.code = code;
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
  return new CareRequestApiError(message, code);
}

/** Lists the tenant's pending parent care-schedule change requests. */
export async function listCareScheduleChangeRequests(): Promise<
  StaffCareRequest[]
> {
  const response = await fetch("/api/students/care-schedule-change-requests", {
    method: "GET",
    headers: { "Content-Type": "application/json" },
    cache: "no-store",
  });
  if (!response.ok) {
    throw await readError(
      response,
      "Betreuungszeit-Anfragen konnten nicht geladen werden",
    );
  }
  return unwrap((await response.json()) as Envelope<StaffCareRequest[]>);
}

/** Approves (applies the weekly plan) or rejects one care-schedule request. */
export async function decideCareScheduleChangeRequest(
  requestId: string,
  approve: boolean,
  reason?: string,
): Promise<StaffCareRequest> {
  const response = await fetch(
    `/api/students/care-schedule-change-requests/${encodeURIComponent(requestId)}/decide`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ approve, reason: reason ?? "" }),
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
