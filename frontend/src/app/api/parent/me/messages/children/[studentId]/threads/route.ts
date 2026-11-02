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
  audience: "team" | "leadership";
  readers: { school: { kind: string; names: string[] } };
}

/**
 * Proxy GET /api/parent/me/messages/children/{studentId}/threads → backend.
 * Returns the guardian's conversation summary/summaries for one child (at most
 * one per audience: team and leadership, #5200), WITHOUT marking the thread read — the child detail
 * page uses this instead of fetching the whole inbox. Guardian ownership is
 * enforced server-side.
 */
export const GET = proxyGet<BackendThreadSummary[]>(
  (params) =>
    `/parent/me/messages/children/${requirePathSegmentParam(params, "studentId")}/threads`,
);
