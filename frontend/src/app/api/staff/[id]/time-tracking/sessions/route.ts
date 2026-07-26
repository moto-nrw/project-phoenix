import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * POST /api/staff/[id]/time-tracking/sessions
 * Admin "nachtragen" endpoint: create a work session for the named staff
 * member. Gated server-side on time_tracking:manage.
 */
export const POST = proxyPost(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/time-tracking/sessions`,
);
