/**
 * Staff client for the parent Stammdaten change-request review queue.
 * Calls the Next.js proxy routes under /api/students/master-data-change-requests
 * which forward (with the tenant session token) to the backend.
 */

import {
  apiErrorFromResponse,
  type ApiError,
  transportFetch,
} from "~/lib/api-error";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "MasterDataReviewAPI" });

type ChangeRequestStatus = "auto_applied" | "pending" | "approved" | "rejected";

// One pending change request in the staff queue. Mirrors
// api/students.MasterDataChangeRequestResponse.
export interface StaffMasterDataChange {
  readonly id: string;
  readonly student_id: string;
  readonly first_name: string;
  readonly last_name: string;
  readonly target: string;
  readonly field_key: string;
  readonly old_value?: unknown;
  readonly new_value: unknown;
  readonly status: ChangeRequestStatus;
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

async function readError(
  response: Response,
  message: string,
): Promise<ApiError> {
  const error = await apiErrorFromResponse(response, message);
  logger.error("master_data_review_request_failed", {
    status: response.status,
    ...(error.code ? { code: error.code } : {}),
  });
  return error;
}

/** Approves (and applies) or rejects one change request. */
export async function decideMasterDataChangeRequest(
  requestId: string,
  approve: boolean,
  reason?: string,
  expectedVersion?: string,
): Promise<StaffMasterDataChange> {
  const response = await transportFetch(
    `/api/students/master-data-change-requests/${encodeURIComponent(requestId)}/decide`,
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
    throw await readError(response, "master data decision failed");
  }
  return unwrap((await response.json()) as Envelope<StaffMasterDataChange>);
}

/**
 * One decided Stammdaten change request in the staff history. Mirrors
 * api/students.MasterDataChangeRequestHistoryResponse.
 */
export interface StaffMasterDataHistoryEntry extends StaffMasterDataChange {
  readonly decided_at: string;
  /** Absent for auto-applied rows (no reviewer). */
  readonly decided_by_name?: string;
  readonly review_reason?: string;
}
