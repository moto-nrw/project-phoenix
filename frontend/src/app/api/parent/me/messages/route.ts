import {
  createParentGetHandler,
  parentApiGet,
} from "~/lib/parent/route-wrapper.server";

interface BackendThreadSummary {
  thread_id: string;
  subject: string;
  student_id: string;
  student_name: string;
  last_message_at?: string;
  last_sender_kind?: "guardian" | "staff";
  last_message_body?: string;
  unread: number;
}

/**
 * Proxy GET /api/parent/me/messages → backend. Returns one summary per thread
 * (mailbox-style), with the guardian's unread (staff-sent) count.
 */
export const GET = createParentGetHandler<BackendThreadSummary[]>(
  async (_request, token) => {
    return parentApiGet<BackendThreadSummary[]>(`/parent/me/messages`, token);
  },
);
