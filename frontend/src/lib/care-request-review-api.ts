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
  readonly diff: readonly RequestDiffEntry[];
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

async function readError(response: Response, fallback: string): Promise<Error> {
  let message = fallback;
  try {
    const body = (await response.json()) as { error?: string };
    if (body.error) message = body.error;
  } catch {
    // not JSON
  }
  logger.error("care_request_review_request_failed", {
    status: response.status,
    message,
  });
  return new Error(message);
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
