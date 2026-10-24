import { proxyPost } from "~/lib/route-proxy.server";

/**
 * Proxy POST /api/messages/group-threads/{groupThreadId}/messages → backend
 * (#4784): one more message to the families the target reaches today.
 */
export const POST = proxyPost(
  (params) =>
    `/api/messages/group-threads/${encodeURIComponent(String(params.groupThreadId))}/messages`,
);
