import { proxyPost } from "~/lib/route-wrapper.server";

/**
 * Proxy POST /api/messages/group-threads/{groupThreadId}/close → backend
 * (#4784): parents can no longer answer.
 */
export const POST = proxyPost(
  (params) =>
    `/api/messages/group-threads/${encodeURIComponent(String(params.groupThreadId))}/close`,
);
