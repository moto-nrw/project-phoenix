/**
 * Staff client for the post-enrollment offering change-request review queue
 * (#1665). Calls the Next.js proxy routes under
 * /api/students/offering-change-requests which forward (with the tenant session
 * token) to the backend.
 */

import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OfferingRequestReviewAPI" });

type OfferingRequestStatus = "pending" | "approved" | "rejected" | "withdrawn";

/** One "current → requested" line, pre-rendered in German by the backend. */
export interface OfferingRequestDiffLine {
  readonly offering_id: string;
  readonly label: string;
  readonly old: string;
  readonly new: string;
  /** True when a Mitbuchungs-Regel (or the required lunch) added days here. */
  readonly automatic?: boolean;
  /** German day list of the automatic share ("Do, Fr"). */
  readonly automatic_days?: string;
  /** Rule-derived part of automatic_days, excluding required-lunch days. */
  readonly rule_days?: string;
  /** Materialized German day list after this co-booking rule is suppressed. */
  readonly new_when_excluded?: string;
  /** Offerings whose rule produced the automatic share (for cascade greying). */
  readonly trigger_ids?: readonly string[];
  readonly trigger_names?: readonly string[];
  /** True when staff may exclude this rule-added line per request (#2370). */
  readonly optoutable?: boolean;
}

/** One booking the request leaves exactly as it is. */
interface OfferingRequestUnchangedLine {
  readonly offering_id: string;
  readonly label: string;
  readonly days: string;
}

/**
 * One offering change request in the staff queue. Mirrors
 * api/students.OfferingRequestResponse.
 */
export interface StaffOfferingRequest {
  readonly id: string;
  readonly student_id: string;
  readonly student_name: string;
  readonly status: OfferingRequestStatus;
  /** Date the switch would take effect (YYYY-MM-DD). */
  readonly effective_from: string;
  readonly note?: string;
  readonly diff: readonly OfferingRequestDiffLine[];
  /** Bookings this request does not touch, for the complete after picture. */
  readonly unchanged?: readonly OfferingRequestUnchangedLine[];
  /** True when approving would leave the child without any offering (#2434). */
  readonly full_withdrawal?: boolean;
  readonly reason?: string;
  readonly created_at: string;
  readonly reviewed_at?: string;
}

export interface OfferingRequestPreviewSelection {
  readonly offering_id: string;
  readonly new: string;
  readonly removed?: boolean;
}

export interface OfferingRequestPreview {
  readonly selections: readonly OfferingRequestPreviewSelection[];
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
 * Error thrown by the offering-request client. Carries the backend's stable
 * conflict `code` (e.g. "offering_change_capacity_full",
 * "change_request_not_pending") so the review UI can name the concrete recovery
 * action instead of collapsing every failure into one message.
 */
export class OfferingRequestApiError extends Error {
  readonly code?: string;
  constructor(message: string, code?: string) {
    super(message);
    this.name = "OfferingRequestApiError";
    this.code = code;
  }
}

async function readError(
  response: Response,
  fallback: string,
): Promise<OfferingRequestApiError> {
  let message = fallback;
  let code: string | undefined;
  try {
    const body = (await response.json()) as { error?: string; code?: string };
    if (body.error) message = body.error;
    if (body.code) code = body.code;
  } catch {
    // not JSON
  }
  logger.error("offering_request_review_request_failed", {
    status: response.status,
    message,
    ...(code ? { code } : {}),
  });
  return new OfferingRequestApiError(message, code);
}

/**
 * Approves (applies the dated switch) or rejects one offering change request.
 * Approval can legitimately fail (capacity, deactivated offering); the row then
 * stays pending on purpose.
 */
export async function decideOfferingChangeRequest(
  requestId: string,
  approve: boolean,
  reason?: string,
  excludedOfferingIds?: readonly string[],
): Promise<void> {
  const response = await fetch(
    `/api/students/offering-change-requests/${encodeURIComponent(requestId)}/decide`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        approve,
        reason: reason ?? "",
        excluded_offering_ids: excludedOfferingIds,
      }),
    },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Entscheidung konnte nicht gespeichert werden",
    );
  }
}

/** Materializes the review card with its current co-booking overrides. */
export async function previewOfferingChangeRequest(
  requestId: string,
  excludedOfferingIds: readonly string[],
): Promise<OfferingRequestPreview> {
  const response = await fetch(
    `/api/students/offering-change-requests/${encodeURIComponent(requestId)}/preview`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        excluded_offering_ids: excludedOfferingIds,
      }),
    },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Vorschau konnte nicht aktualisiert werden",
    );
  }
  return unwrap((await response.json()) as Envelope<OfferingRequestPreview>);
}

/**
 * One decided offering change request in the staff history. Mirrors
 * api/students.OfferingRequestHistoryResponse; the diff is the frozen
 * decision snapshot, never recomputed against current bookings.
 */
export interface StaffOfferingRequestHistoryEntry extends StaffOfferingRequest {
  readonly decided_at: string;
  /** Absent for withdrawn rows (no reviewer). */
  readonly decided_by_name?: string;
  /** Payload-derived recap when no frozen decision snapshot exists. */
  readonly requested?: readonly OfferingRequestRequestedLine[];
}

interface OfferingRequestRequestedLine {
  readonly offering_id: string;
  readonly label: string;
  readonly new: string;
}
