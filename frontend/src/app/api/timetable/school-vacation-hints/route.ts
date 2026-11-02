import { proxyGet } from "~/lib/route-proxy.server";

/**
 * GET /api/timetable/school-vacation-hints
 * Ferien der nächsten acht Wochen mit geplanten Regelterminen (#5345).
 */
export const GET = proxyGet("/api/timetable/school-vacation-hints");
