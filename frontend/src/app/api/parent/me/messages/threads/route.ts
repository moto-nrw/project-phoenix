import {
  createParentPostHandler,
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

interface StartThreadBody {
  student_id: string;
  subject: string;
  body: string;
}

/**
 * Proxy POST /api/parent/me/messages/threads → backend. Starts a new
 * conversation thread (subject + first message) about one child and returns
 * the created thread. Guardian check + feature gate happen server-side.
 */
export const POST = createParentPostHandler<BackendThreadView, StartThreadBody>(
  async (_request, body, token) => {
    return parentApiPost<BackendThreadView, StartThreadBody>(
      `/parent/me/messages/threads`,
      token,
      body,
    );
  },
);
