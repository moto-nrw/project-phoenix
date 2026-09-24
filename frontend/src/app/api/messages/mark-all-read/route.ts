import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy POST /api/messages/mark-all-read → backend. Marks every conversation
 * the caller sees as unread as read for the caller's own account.
 */
export const POST = createPostHandler(
  async (_request: NextRequest, _body: unknown, token: string) => {
    await apiPost("/api/messages/mark-all-read", token, {});
    return null;
  },
);
