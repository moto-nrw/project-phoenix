import { proxyGet, proxyPost } from "~/lib/route-proxy.server";

/**
 * Proxy /api/surveys → backend (#4781). GET lists the school's Umfragen,
 * POST creates a draft. Both need surveys:manage; the backend checks it.
 */
export const GET = proxyGet(() => "/api/surveys");
export const POST = proxyPost("/api/surveys");
