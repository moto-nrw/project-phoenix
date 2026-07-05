import { proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * PUT /api/staff/[id]/time-tracking/sessions/[sessionId]
 * Admin edit endpoint: correct an existing work session for the named
 * staff member. Gated server-side on time_tracking:manage.
 */
export const PUT = proxyPut(
  (p) =>
    `/api/staff/${requirePathSegmentParam(p)}/time-tracking/sessions/${requirePathSegmentParam(p, "sessionId")}`,
);
