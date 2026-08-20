/**
 * Staff client for the parent absence-request review queue. Calls the Next.js proxy routes under
 * /api/students/excused-absence-requests which forward (with the tenant session
 * token) to the backend. Mirrors care-request-review-api.ts.
 */

import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ExcusedRequestReviewAPI" });

type ExcusedRequestStatus = "pending" | "approved" | "rejected" | "withdrawn";

// One absence request in the staff queue. Mirrors
// api/students.StaffExcusedRequestResponse. `dates` are YYYY-MM-DD.
export interface StaffExcusedRequest {
  readonly id: string;
  readonly student_id: string;
  readonly first_name: string;
  readonly last_name: string;
  readonly absence_status: "sick" | "excused";
  readonly status: ExcusedRequestStatus;
  readonly dates: readonly string[];
  readonly note: string;
  readonly reason?: string;
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
 * Error thrown by the excused-request client. Carries the backend's stable 409
 * `code` (e.g. "change_request_not_pending") so the review UI can name the
 * concrete recovery action instead of collapsing every failure into one generic
 * message. The raw `error` string stays the Error message for logging.
 */
export class ExcusedRequestApiError extends Error {
  readonly code?: string;
  constructor(message: string, code?: string) {
    super(message);
    this.name = "ExcusedRequestApiError";
    this.code = code;
  }
}

async function readError(
  response: Response,
  fallback: string,
): Promise<ExcusedRequestApiError> {
  let message = fallback;
  let code: string | undefined;
  try {
    const body = (await response.json()) as { error?: string; code?: string };
    if (body.error) message = body.error;
    if (body.code) code = body.code;
  } catch {
    // not JSON
  }
  logger.error("excused_request_review_request_failed", {
    status: response.status,
    message,
    ...(code ? { code } : {}),
  });
  return new ExcusedRequestApiError(message, code);
}

/** Approves (marks the child absent) or rejects one absence request. */
export async function decideExcusedAbsenceRequest(
  requestId: string,
  approve: boolean,
  reason?: string,
): Promise<StaffExcusedRequest> {
  const response = await fetch(
    `/api/students/excused-absence-requests/${encodeURIComponent(requestId)}/decide`,
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
  return unwrap((await response.json()) as Envelope<StaffExcusedRequest>);
}

/**
 * One decided absence request in the staff history. Mirrors
 * api/students.StaffExcusedRequestHistoryResponse.
 */
export interface StaffExcusedRequestHistoryEntry extends StaffExcusedRequest {
  readonly decided_at: string;
  /** Absent for withdrawn rows (no reviewer). */
  readonly decided_by_name?: string;
}
