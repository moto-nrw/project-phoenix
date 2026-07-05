import { proxyGet } from "~/lib/route-proxy.server";

/**
 * Handler for GET /api/active/groups/unclaimed
 * Returns all active groups that have no supervisors assigned
 * Used for deviceless rooms like Schulhof where teachers claim via frontend
 */
export const GET = proxyGet("/api/active/groups/unclaimed", { raw: true });
