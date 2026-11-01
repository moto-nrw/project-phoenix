import { proxyGet } from "~/lib/route-proxy.server";

/**
 * GET /api/staff/qualifications
 * Die Nachweise aller Mitarbeitenden mit Status (#4808).
 */
export const GET = proxyGet("/api/staff/qualifications");
