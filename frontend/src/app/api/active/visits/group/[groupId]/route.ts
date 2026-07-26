// app/api/active/visits/group/[groupId]/route.ts
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Handler for GET /api/active/visits/group/[groupId]
 * Returns visits for a specific group
 */
export const GET = proxyGet(
  (p) => `/api/active/visits/group/${requirePathSegmentParam(p, "groupId")}`,
  { raw: true },
);
