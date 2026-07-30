import { proxyGet } from "~/lib/parent/route-wrapper.server";

interface UnreadCount {
  unread_count: number;
}

/**
 * Proxy GET /api/parent/me/feedback/unread-count → backend. Sums unread
 * product-team replies across every board the guardian has, for the sidebar
 * badge.
 */
export const GET = proxyGet<UnreadCount>("/parent/me/feedback/unread-count");
