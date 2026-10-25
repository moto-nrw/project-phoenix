import { proxyGet } from "~/lib/route-proxy.server";

/**
 * GET /api/timetable/holidays?from=&to=
 * Gesetzliche Feiertage des Bundeslands der Schule für „Mein Kalender“ und
 * „Schuljahr und Kalender“ (#4775). Lesbar mit calendar:own oder
 * schedules:read, ohne Recht auf die Zeiterfassung.
 */
export const GET = proxyGet("/api/timetable/holidays");
