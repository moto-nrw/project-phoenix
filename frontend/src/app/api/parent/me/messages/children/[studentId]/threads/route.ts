import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface BackendThreadSummary {
  thread_id: string;
  student_id: string;
  student_name: string;
  school_name: string;
  counterpart_name: string;
  last_message_at?: string;
  last_sender_kind?: "guardian" | "staff";
  last_message_body?: string;
  last_message_payload?: Record<string, unknown>;
  unread: number;
}

/**
 * Proxy GET /api/parent/me/messages/children/{studentId}/threads → backend.
 * Returns the guardian's conversation summary/summaries for one child (at most
 * one per the chat model), WITHOUT marking the thread read — the child detail
 * page uses this instead of fetching the whole inbox. Guardian ownership is
 * enforced server-side.
 */
export const GET = proxyGet<BackendThreadSummary[]>(
  (params) =>
    `/parent/me/messages/children/${requirePathSegmentParam(params, "studentId")}/threads`,
);
