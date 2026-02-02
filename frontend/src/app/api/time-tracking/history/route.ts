import { createProxyGetDataHandler } from "~/lib/route-wrapper";

/**
 * GET /api/time-tracking/history?from=YYYY-MM-DD&to=YYYY-MM-DD
 * Get work session history for date range
 */
export const GET = createProxyGetDataHandler("/api/time-tracking/history");
