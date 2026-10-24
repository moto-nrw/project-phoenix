import { proxyGet } from "~/lib/parent/route-wrapper.server";

interface BackendGroupThreadSummary {
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
}

/**
 * Proxy GET /api/parent/me/messages/group-threads → backend. Returns the group
 * threads (#4784) that reached the guardian, for their own children only.
 */
export const GET = proxyGet<BackendGroupThreadSummary[]>(
  "/parent/me/messages/group-threads",
);
