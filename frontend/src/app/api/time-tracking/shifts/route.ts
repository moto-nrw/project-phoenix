import { proxyGet } from "~/lib/route-proxy.server";

/**
 * GET /api/time-tracking/shifts?from=YYYY-MM-DD&to=YYYY-MM-DD
 * The authenticated staff member's own planned shifts (Dienstplan).
 */
export const GET = proxyGet("/api/time-tracking/shifts");
