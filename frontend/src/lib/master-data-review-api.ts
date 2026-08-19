/**
 * Staff client for the parent Stammdaten change-request review queue.
 * Calls the Next.js proxy routes under /api/students/master-data-change-requests
 * which forward (with the tenant session token) to the backend.
 */

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

async function readError(response: Response, fallback: string): Promise<Error> {
  let message = fallback;
  try {
    const body = (await response.json()) as { error?: string };
    if (body.error) message = body.error;
  } catch {
    // not JSON
  }
  logger.error("master_data_review_request_failed", {
    status: response.status,
    message,
  });
  return new Error(message);
}

/** Lists the tenant's pending parent change requests. */
export async function listMasterDataChangeRequests(): Promise<
  StaffMasterDataChange[]
> {
  const response = await fetch("/api/students/master-data-change-requests", {
    method: "GET",
    headers: { "Content-Type": "application/json" },
    cache: "no-store",
  });
  if (!response.ok) {
    throw await readError(
      response,
      "Änderungsanfragen konnten nicht geladen werden",
    );
  }
  return unwrap((await response.json()) as Envelope<StaffMasterDataChange[]>);
}

/** Approves (and applies) or rejects one change request. */
export async function decideMasterDataChangeRequest(
  requestId: string,
  approve: boolean,
  reason?: string,
): Promise<StaffMasterDataChange> {
  const response = await fetch(
    `/api/students/master-data-change-requests/${encodeURIComponent(requestId)}/decide`,
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
  return unwrap((await response.json()) as Envelope<StaffMasterDataChange>);
}

/** One page of the decided-request history. */
export interface HistoryPage<T> {
  readonly items: readonly T[];
  /** Absent on the last page. */
  readonly next_cursor?: string;
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

/** Lists the tenant's decided parent change requests, newest decision first. */
export async function listMasterDataChangeRequestHistory(
  cursor?: string,
): Promise<HistoryPage<StaffMasterDataHistoryEntry>> {
  const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : "";
  const response = await fetch(
    `/api/students/master-data-change-requests/history${query}`,
    {
      method: "GET",
      headers: { "Content-Type": "application/json" },
      cache: "no-store",
    },
  );
  if (!response.ok) {
    throw await readError(response, "Historie konnte nicht geladen werden");
  }
  return unwrap(
    (await response.json()) as Envelope<
      HistoryPage<StaffMasterDataHistoryEntry>
    >,
  );
}
