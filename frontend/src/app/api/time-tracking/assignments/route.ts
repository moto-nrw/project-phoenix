import { proxyGet } from "~/lib/route-proxy.server";

/**
 * GET /api/time-tracking/assignments?from=YYYY-MM-DD&to=YYYY-MM-DD
 * The authenticated staff member's own Betreuungsplan blocks (room + activity +
 * Vertretungsplan state) for the range — "Mein Tag" (#1844).
 */
export const GET = proxyGet("/api/time-tracking/assignments");
