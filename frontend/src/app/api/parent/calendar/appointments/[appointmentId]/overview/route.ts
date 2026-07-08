import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy GET /api/parent/calendar/appointments/{appointmentId}/overview →
 * backend. The route-wrapper injects the parent session token + 401 retry
 * with a refreshed token.
 */
export const GET = proxyGet<unknown>(
  (params) =>
    `/parent/me/calendar/appointments/${requirePathSegmentParam(params, "appointmentId")}/overview`,
);
