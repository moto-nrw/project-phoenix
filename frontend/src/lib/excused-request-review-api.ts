import { ApiError, apiErrorFromBody, transportFetch } from "./api-error";
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
  /**
   * Was für die beantragten Tage aktuell eingetragen ist (#2267), je Tag
   * `YYYY-MM-DD` → `present | sick | excused | class_trip`. Damit die
   * Entscheidung „Aktuell → Gewünscht" zeigen kann, statt nur den Wunsch.
   */
  readonly current_status_by_date?: Readonly<Record<string, string>>;
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
 * (e.g. "students.change_request_not_pending") lets the review UI name the
 * concrete recovery action; `detail` stays diagnostics for the log.
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
  logger.error("excused_request_review_request_failed", {
    status: response.status,
    message: error.message,
    ...(error.code ? { code: error.code } : {}),
  });
  return error;
}

/** Approves (marks the child absent) or rejects one absence request. */
export async function decideExcusedAbsenceRequest(
  requestId: string,
  approve: boolean,
  reason?: string,
  expectedVersion?: string,
): Promise<StaffExcusedRequest> {
  const response = await transportFetch(
    `/api/students/excused-absence-requests/${encodeURIComponent(requestId)}/decide`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        approve,
        reason: reason ?? "",
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
