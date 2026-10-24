import { proxyGet } from "~/lib/route-proxy.server";

/**
 * Proxy GET /api/messages/reachable-students → backend. Returns the IDs of
 * the children with at least one guardian who can receive a message, so
 * „Neue Nachricht“ can mark the others in its child list (#4362).
 */
export const GET = proxyGet(() => "/api/messages/reachable-students");
