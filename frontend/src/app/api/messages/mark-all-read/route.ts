import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy POST /api/messages/mark-all-read → backend. Clears the caller's own
 * unread numbers for every conversation they see as unread and returns the
 * caller's new unread count. Parents get no read receipt from it (#3673).
 */
export const POST = createPostHandler(
  async (_request: NextRequest, _body: unknown, token: string) => {
    const response = await apiPost<{ data: { unread_count: number } }>(
      "/api/messages/mark-all-read",
      token,
      {},
    );
    return response.data;
  },
);
