import { proxyGet } from "~/lib/route-proxy.server";

/**
 * GET /api/time-tracking/closing-days?from=&to=
 * OGS-Schließtage des Tenants (schedule.closing_days) für die
 * Kalender-Markierung in der Zeiterfassung (#1418 3b).
 */
export const GET = proxyGet("/api/time-tracking/closing-days");
