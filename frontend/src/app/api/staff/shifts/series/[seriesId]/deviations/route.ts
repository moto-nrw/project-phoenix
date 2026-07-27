import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * GET /api/staff/shifts/series/{seriesId}/deviations
 * Which upcoming occurrences were adjusted individually (#2028): the ones a
 * whole-series edit rebuilds, and the ones it leaves alone. Read before the
 * edit is written so the planner is told what changes.
 */
export const GET = proxyGet(
  (p) =>
    `/api/staff-shifts/series/${requirePathSegmentParam(p, "seriesId")}/deviations`,
);
