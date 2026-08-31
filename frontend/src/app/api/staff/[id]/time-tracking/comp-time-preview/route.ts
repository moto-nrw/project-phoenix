import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * GET /api/staff/[id]/time-tracking/comp-time-preview?date_start=&date_end=&half_day=
 * Stundenkonto projection for a planned Freizeitausgleich (#2873): current
 * balance, planned deduction, and the expected balance after the entry.
 * Gated server-side on time_tracking:manage.
 */
export const GET = proxyGet(
  (p) =>
    `/api/staff/${requirePathSegmentParam(p)}/time-tracking/comp-time-preview`,
);
