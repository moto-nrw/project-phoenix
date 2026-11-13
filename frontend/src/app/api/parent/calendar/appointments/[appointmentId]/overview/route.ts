import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy GET /api/parent/calendar/appointments/{appointmentId}/overview →
 * backend. Parents get the answer counts and the staff by name, never other
 * parents (#6846).
 */
export const GET = proxyGet<unknown>(
  (params) =>
    `/parent/me/calendar/appointments/${requirePathSegmentParam(params, "appointmentId")}/overview`,
);
