import { proxyGet } from "~/lib/route-proxy.server";

/**
 * GET /api/time-tracking/month-summary?year=&month=
 * Own Monatskarte (#1842).
 */
export const GET = proxyGet("/api/time-tracking/month-summary");
