import { proxyGet } from "~/lib/route-proxy.server";

/**
 * Proxy GET /api/messages/group-threads/{groupThreadId} → backend (#4784):
 * the messages and which families have read them.
 */
export const GET = proxyGet(
  (params) =>
    `/api/messages/group-threads/${encodeURIComponent(String(params.groupThreadId))}`,
);
