// Cross-staff audit feed (#1417). Gated on time_tracking:manage
// backend-side: every event names an affected and an acting person plus
// free-text reasons.
import { proxyGet } from "~/lib/route-proxy.server";
import type { BackendAuditLogPage } from "~/lib/staff-audit-log-api";

/**
 * GET /api/staff/time-tracking/audit-log
 *   ?from=&to=&staff_id=&actor_id=&sources=&cursor=&limit=
 */
export const GET = proxyGet<BackendAuditLogPage>(
  "/api/staff/time-tracking/audit-log",
);
