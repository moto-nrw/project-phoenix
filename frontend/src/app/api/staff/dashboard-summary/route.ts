// Tenant-wide Leitungs-Dashboard aggregate (#1417 Tranche 2a). Permission
// users:read — the aggregated KPIs carry no per-person working time, unlike
// /api/staff/time-tracking/overview.
import { proxyGet } from "~/lib/route-proxy.server";
import type { BackendDashboardSummary } from "~/lib/staff-overview-api";

/** GET /api/staff/dashboard-summary?period=week|month */
export const GET = proxyGet<BackendDashboardSummary>(
  "/api/staff/dashboard-summary",
);
