import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * GET /api/staff/[id]/time-tracking/month-summary?year=&month=
 * Admin Monatskarte for one staff member (#1842). Gated server-side on
 * time_tracking:manage.
 */
export const GET = proxyGet(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/time-tracking/month-summary`,
);
