import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy POST /api/messages/threads/{threadId}/unread → backend. Marks the
 * conversation unread for the whole team until a staff member opens or
 * answers it.
 */
export const POST = createPostHandler(
  async (
    _request: NextRequest,
    _body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const threadId = params.threadId as string;
    if (!threadId) {
      throw new Error("Thread ID is required");
    }
    await apiPost(`/api/messages/threads/${threadId}/unread`, token, {});
    return null;
  },
);
