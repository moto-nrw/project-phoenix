/**
 * Staff client for the combined Änderungsanfragen queue. The review lists have
 * their own per-queue clients (care-request-review-api, master-data-review-*);
 * this module only exposes the cross-queue pending count that drives the
 * sidebar badge.
 */

import {
  isParentRequestReviewAccess,
  isStudentRequestReviewCoverage,
  type ParentRequestReviewAccess,
  type StudentRequestReviewCoverage,
} from "~/lib/change-request-access";

interface Envelope<T> {
  readonly data?: T;
}

/**
 * Fetches the combined number of pending parent change requests (master-data +
 * care schedule) for the tenant. Mirrors fetchUnreadCount: returns 0 on any
 * failure so a badge fetch can never break the sidebar.
 */
export async function fetchPendingChangeRequestCount(): Promise<number> {
  const response = await fetch("/api/students/change-requests/pending-count", {
    method: "GET",
    headers: { "Content-Type": "application/json" },
    cache: "no-store",
  });
  if (!response.ok) return 0;
  const json = (await response.json()) as Envelope<{ pending_count: number }>;
  return json.data?.pending_count ?? 0;
}

export async function fetchChangeRequestAccess(): Promise<ParentRequestReviewAccess> {
  const response = await fetch("/api/students/change-requests/access", {
    method: "GET",
    headers: { "Content-Type": "application/json" },
    cache: "no-store",
  });
  if (!response.ok) {
    throw new Error(`Change request access failed: ${response.status}`);
  }
  const json = (await response.json()) as Envelope<{
    review_access?: unknown;
  }>;
  const access = json.data?.review_access;
  if (!isParentRequestReviewAccess(access)) {
    throw new Error("Change request access response is invalid");
  }
  return access;
}

/**
 * Fragt, ob der Prüfbereich der angemeldeten Person ein Kind abdeckt (#3886).
 * Der Nachrichtenverlauf bietet „Anfrage ansehen“ nur dann an; sonst endete
 * der Klick in einem 403 der Detailansicht.
 */
export async function fetchStudentRequestReviewCoverage(
  studentId: string,
): Promise<StudentRequestReviewCoverage> {
  const response = await fetch(
    `/api/students/change-requests/access?student_id=${encodeURIComponent(studentId)}`,
    {
      method: "GET",
      headers: { "Content-Type": "application/json" },
      cache: "no-store",
    },
  );
  if (!response.ok) {
    throw new Error(
      `Student request review coverage failed: ${response.status}`,
    );
  }
  const json = (await response.json()) as Envelope<{ student?: unknown }>;
  const coverage = json.data?.student;
  if (!isStudentRequestReviewCoverage(coverage)) {
    throw new Error("Student request review coverage response is invalid");
  }
  return coverage;
}
