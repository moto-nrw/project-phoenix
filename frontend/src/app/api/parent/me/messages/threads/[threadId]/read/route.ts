import {
  createParentPostHandler,
  parentApiPost,
} from "~/lib/parent/route-wrapper.server";

interface MarkReadResult {
  ok: boolean;
}

/**
 * Proxy POST /api/parent/me/messages/threads/{threadId}/read → backend.
 * Advances the guardian's read cursor for the thread. The request body is
 * empty — an empty object is forwarded.
 */
export const POST = createParentPostHandler<MarkReadResult, unknown>(
  async (_request, _body, token, params) => {
    const threadId = String(params.threadId);
    return parentApiPost<MarkReadResult, Record<string, never>>(
      `/parent/me/messages/threads/${encodeURIComponent(threadId)}/read`,
      token,
      {},
    );
  },
);
