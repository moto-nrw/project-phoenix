import { proxyGet, proxyPost } from "~/lib/route-proxy.server";

/**
 * Proxy /api/messages/group-threads → backend (#4784). GET lists the
 * school's Gruppennachrichten, POST starts one with its first message.
 */
export const GET = proxyGet(() => "/api/messages/group-threads");
export const POST = proxyPost("/api/messages/group-threads");
