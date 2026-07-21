import { proxyGet } from "~/lib/route-proxy.server";

/**
 * GET /api/time-tracking/holidays?from=&to=
 * Gesetzliche Feiertage des Tenants (Bundesland-Setting) für die
 * Kalender-Markierung und die Feiertagsarbeit-Warnung (#1418 3a).
 */
export const GET = proxyGet("/api/time-tracking/holidays");
