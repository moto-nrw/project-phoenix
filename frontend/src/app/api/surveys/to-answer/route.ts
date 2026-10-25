import { proxyGet } from "~/lib/route-proxy.server";

/**
 * Proxy GET /api/surveys/to-answer → backend (#4781): the open staff
 * Umfragen the signed-in staff member can answer, with their own
 * participation flag only.
 */
export const GET = proxyGet(() => "/api/surveys/to-answer");
