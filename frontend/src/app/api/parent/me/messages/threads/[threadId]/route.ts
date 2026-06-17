import {
  createParentGetHandler,
  createParentPostHandler,
  parentApiGet,
  parentApiPost,
} from "~/lib/parent/route-wrapper.server";

interface BackendMessage {
  id: string;
  sender_kind: "guardian" | "staff";
  sender_name: string;
  body: string;
  created_at: string;
}

interface BackendThreadView {
  thread_id: string;
  subject: string;
  student_id: string;
  student_name: string;
  messages: BackendMessage[];
}

interface PostMessageBody {
  body: string;
}

/**
 * Proxy GET /api/parent/me/messages/threads/{threadId} → backend. Returns the
 * full conversation thread (messages oldest-first). Guardian check happens
 * server-side.
 */
export const GET = createParentGetHandler<BackendThreadView>(
  async (_request, token, params) => {
    const threadId = String(params.threadId);
    return parentApiGet<BackendThreadView>(
      `/parent/me/messages/threads/${encodeURIComponent(threadId)}`,
      token,
    );
  },
);

/**
 * Proxy POST /api/parent/me/messages/threads/{threadId} → backend. Appends a
 * guardian message to the thread and returns the full updated thread.
 */
export const POST = createParentPostHandler<BackendThreadView, PostMessageBody>(
  async (_request, body, token, params) => {
    const threadId = String(params.threadId);
    return parentApiPost<BackendThreadView, PostMessageBody>(
      `/parent/me/messages/threads/${encodeURIComponent(threadId)}`,
      token,
      body,
    );
  },
);
