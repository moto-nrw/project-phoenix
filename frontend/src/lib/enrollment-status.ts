/**
 * Status rules of an enrollment child and an enrollment change request. The
 * backend owns them (`backend/modules/enrollment/request_child.go`,
 * `open_enrollments.go`); every screen reads them from here so a new status
 * lands in one place.
 */

export type ChildStatus =
  | "submitted"
  | "under_review"
  | "approved"
  | "waitlisted"
  | "rejected"
  | "withdrawn"
  | "pending_renewal"
  | "auto_renewed"
  | "pending_admin_review";

export type ChangeRequestStatus =
  | "pending_review"
  | "needs_parent_response"
  | "approved"
  | "rejected"
  | "cancelled";

const TERMINAL_CHILD_STATUSES: ReadonlySet<string> = new Set<ChildStatus>([
  "approved",
  "rejected",
  "withdrawn",
]);

const OPEN_CHILD_STATUSES: ReadonlySet<string> = new Set<ChildStatus>([
  "submitted",
  "under_review",
  "pending_admin_review",
]);

const OPEN_CHANGE_REQUEST_STATUSES: ReadonlySet<string> =
  new Set<ChangeRequestStatus>(["pending_review", "needs_parent_response"]);

/** Decided for good: approved, rejected or withdrawn (`IsTerminal`). */
export function isTerminalChildStatus(status: string): boolean {
  return TERMINAL_CHILD_STATUSES.has(status);
}

/**
 * Waits for a decision of the school, as the school's list counts „Offen“
 * (`OpenEnrollmentChildStatuses`). Waitlisted and renewal states do not.
 */
export function isOpenChildStatus(status: string): boolean {
  return OPEN_CHILD_STATUSES.has(status);
}

/** The school or the family still has to act on the change request. */
export function isOpenChangeRequestStatus(status: string): boolean {
  return OPEN_CHANGE_REQUEST_STATUSES.has(status);
}

export interface ChildStatusCounts {
  readonly total: number;
  readonly open: number;
  readonly approved: number;
  readonly rejected: number;
}

export function countChildStatuses(
  children: Iterable<{ readonly status: string }>,
): ChildStatusCounts {
  let total = 0;
  let open = 0;
  let approved = 0;
  let rejected = 0;
  for (const child of children) {
    total += 1;
    if (isOpenChildStatus(child.status)) open += 1;
    if (child.status === "approved") approved += 1;
    if (child.status === "rejected") rejected += 1;
  }
  return { total, open, approved, rejected };
}
