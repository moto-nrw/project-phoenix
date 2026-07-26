import { proxyGet } from "~/lib/route-proxy.server";

/**
 * GET /api/time-tracking/schedule-targets?from=&to=
 * Own date-valid Soll per day, feeding the daily table (#1842).
 */
export const GET = proxyGet("/api/time-tracking/schedule-targets");
