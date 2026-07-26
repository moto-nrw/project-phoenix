// Cross-staff Soll/Ist/Saldo/Resturlaub list (#1417 Tranche 2a). Gated on
// time_tracking:manage backend-side: it exposes working-time data of
// identifiable people, unlike the aggregated dashboard summary.
import { proxyGet } from "~/lib/route-proxy.server";
import type { BackendTimeTrackingOverview } from "~/lib/staff-overview-api";

/**
 * GET /api/staff/time-tracking/overview
 *   ?year=&month=&employment_type=&saldo_min=&saldo_max=&sort=&order=
 */
export const GET = proxyGet<BackendTimeTrackingOverview>(
  "/api/staff/time-tracking/overview",
);
