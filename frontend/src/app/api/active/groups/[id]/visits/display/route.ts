// app/api/active/groups/[id]/visits/display/route.ts
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Handler for GET /api/active/groups/[id]/visits/display
 * Returns visits with display data for a specific active group
 * Optimized bulk endpoint for SSE real-time updates
 */
export const GET = proxyGet(
  (p) => `/api/active/groups/${requirePathSegmentParam(p)}/visits/display`,
);
