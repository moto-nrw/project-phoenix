import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * GET /api/time-tracking/{id}/edits
 * Fetch audit trail for a work session
 */
export const GET = proxyGet(
  (p) => `/api/time-tracking/${requirePathSegmentParam(p)}/edits`,
);
