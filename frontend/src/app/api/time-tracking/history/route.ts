import { proxyGet } from "~/lib/route-proxy.server";

/**
 * GET /api/time-tracking/history?from=YYYY-MM-DD&to=YYYY-MM-DD
 * Get work session history for date range
 */
export const GET = proxyGet("/api/time-tracking/history");
