import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface BackendGroupThreadView {
  id: string;
  title: string;
  target_label: string;
  school_name: string;
  counterpart_name: string;
  closed: boolean;
  last_message_at?: string;
  last_message_body: string;
  unread: number;
  children: { student_id: string; first_name: string; last_name: string }[];
  messages: { id: string; body: string; created_at: string }[];
}

/**
 * Proxy GET /api/parent/me/messages/group-threads/{groupThreadId} → backend.
 * Returns the OGS messages that reached the guardian and marks them read.
 */
export const GET = proxyGet<BackendGroupThreadView>(
  (params) =>
    `/parent/me/messages/group-threads/${requirePathSegmentParam(params, "groupThreadId")}`,
);
