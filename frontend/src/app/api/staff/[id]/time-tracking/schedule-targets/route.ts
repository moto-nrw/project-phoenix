import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * GET /api/staff/[id]/time-tracking/schedule-targets?from=&to=
 * Date-valid Soll per day for one staff member (#1842). Gated server-side on
 * time_tracking:manage.
 */
export const GET = proxyGet(
  (p) =>
    `/api/staff/${requirePathSegmentParam(p)}/time-tracking/schedule-targets`,
);
