import { proxyGet } from "~/lib/parent/route-wrapper.server";

interface BackendUnreadCount {
  unread_count: number;
}

/**
 * Proxy GET /api/parent/me/messages/unread-count → backend. Returns the
 * guardian's total count of conversations with unread staff-side activity, for
 * the parents-portal sidebar badge — a light COUNT instead of the full thread
 * list.
 */
export const GET = proxyGet<BackendUnreadCount>(
  "/parent/me/messages/unread-count",
);
