import {
  createParentGetHandler,
  parentApiGet,
} from "~/lib/parent/route-wrapper.server";

interface BackendUnreadCount {
  unread_count: number;
}

/**
 * Proxy GET /api/parent/me/news/unread-count → backend. Returns the guardian's
 * unread parent-news count for the parents-portal Neuigkeiten badge.
 */
export const GET = createParentGetHandler<BackendUnreadCount>(
  async (_request, token) => {
    return parentApiGet<BackendUnreadCount>(
      `/parent/me/news/unread-count`,
      token,
    );
  },
);
