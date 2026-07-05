import { proxyGet } from "~/lib/parent/route-wrapper.server";

interface BackendThreadSummary {
  thread_id: string;
  student_id: string;
  student_name: string;
  school_name: string;
  counterpart_name: string;
  last_message_at?: string;
  last_sender_kind?: "guardian" | "staff";
  last_message_body?: string;
  unread: number;
}

/**
 * Proxy GET /api/parent/me/messages → backend. Returns one summary per child
 * conversation, with the guardian's unread (staff-sent) count.
 */
export const GET = proxyGet<BackendThreadSummary[]>("/parent/me/messages");
