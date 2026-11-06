import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// The backend checks gdpr.attendance_log_enabled and the scope, clamps the
// range and writes the audit row for exactly what it was asked.
export const GET = proxyGet(
  (p) => `/api/students/${requirePathSegmentParam(p)}/attendance-history`,
  { forwardQuery: ["start", "end"] },
);
