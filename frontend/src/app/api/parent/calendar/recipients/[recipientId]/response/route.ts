import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy POST /api/parent/calendar/recipients/{recipientId}/response →
 * backend. The route-wrapper injects the parent session token + 401 retry
 * with a refreshed token.
 */
export const POST = proxyPost<unknown>(
  (params) =>
    `/parent/me/calendar/recipients/${requirePathSegmentParam(params, "recipientId")}/response`,
);
