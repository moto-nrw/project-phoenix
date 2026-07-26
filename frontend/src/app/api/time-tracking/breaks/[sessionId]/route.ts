import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * GET /api/time-tracking/breaks/[sessionId]
 * Get breaks for a specific work session
 */
export const GET = proxyGet(
  (p) => `/api/time-tracking/breaks/${requirePathSegmentParam(p, "sessionId")}`,
);
