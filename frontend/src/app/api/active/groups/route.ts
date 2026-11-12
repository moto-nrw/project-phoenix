// app/api/active/groups/route.ts
// app/api/active/groups/route.ts
import { proxyGet } from "~/lib/route-wrapper.server";

/**
 * Handler for GET /api/active/groups
 * Returns a list of active groups, optionally filtered by query parameters
 */
export const GET = proxyGet("/api/active/groups", { raw: true });
