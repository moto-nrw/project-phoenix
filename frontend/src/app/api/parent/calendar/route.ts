import { proxyGet } from "~/lib/parent/route-wrapper.server";

/**
 * Proxy GET /api/parent/calendar → backend /parent/me/calendar.
 * The route-wrapper handles parent-session auth + 401 retry with a
 * refreshed token, and forwards the query string automatically.
 */
export const GET = proxyGet<unknown>("/parent/me/calendar");
