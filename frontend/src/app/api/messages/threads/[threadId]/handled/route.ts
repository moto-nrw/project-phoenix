import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy POST /api/messages/threads/{threadId}/handled → backend (#6486).
 * Confirms the guardian messages up to the given message for the whole team,
 * without a reply.
 */
export const POST = createPostHandler(
  async (
    _request: NextRequest,
    body: { handled_up_to_message_id?: unknown },
    token: string,
    params: Record<string, unknown>,
  ) => {
    const threadId = params.threadId as string;
    if (!threadId) {
      throw new Error("Thread ID is required");
    }
    await apiPost(
      `/api/messages/threads/${encodeURIComponent(threadId)}/handled`,
      token,
      { handled_up_to_message_id: body?.handled_up_to_message_id },
    );
    return null;
  },
);
