// app/api/active/groups/[id]/visits/route.ts
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Handler for GET /api/active/groups/[id]/visits
 * Returns visits for a specific active group
 */
export const GET = proxyGet(
  (p) => `/api/active/groups/${requirePathSegmentParam(p)}/visits`,
  { raw: true },
);
