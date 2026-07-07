/**
 * Staff client for the combined Änderungsanfragen queue. The review lists have
 * their own per-queue clients (care-request-review-api, master-data-review-*);
 * this module only exposes the cross-queue pending count that drives the
 * sidebar badge.
 */

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
