import { proxyGet } from "~/lib/route-proxy.server";

/**
 * GET /api/staff/me/qualifications
 * Die eigenen Nachweise der angemeldeten Person (#4808).
 */
export const GET = proxyGet("/api/staff/me/qualifications");
