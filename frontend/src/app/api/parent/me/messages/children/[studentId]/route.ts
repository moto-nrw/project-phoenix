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
  student_id: string;
  student_name: string;
  school_name: string;
  counterpart_name: string;
  messages: BackendMessage[];
}

interface PostMessageBody {
  body: string;
}

/**
 * Proxy GET /api/parent/me/messages/children/{studentId} → backend. Returns the
 * guardian's conversation about one child (oldest-first), marking it read.
 * Guardian ownership is enforced server-side.
 */
export const GET = createParentGetHandler<BackendThreadView>(
  async (_request, token, params) => {
    const studentId = String(params.studentId);
    return parentApiGet<BackendThreadView>(
      `/parent/me/messages/children/${encodeURIComponent(studentId)}`,
      token,
    );
  },
);

/**
 * Proxy POST /api/parent/me/messages/children/{studentId} → backend. Appends a
 * guardian message to the child's conversation (created on the first message)
 * and returns the full updated conversation.
 */
export const POST = createParentPostHandler<BackendThreadView, PostMessageBody>(
  async (_request, body, token, params) => {
    const studentId = String(params.studentId);
    return parentApiPost<BackendThreadView, PostMessageBody>(
      `/parent/me/messages/children/${encodeURIComponent(studentId)}`,
      token,
      body,
    );
  },
);
