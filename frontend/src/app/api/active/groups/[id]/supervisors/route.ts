import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Handler for GET /api/active/groups/[id]/supervisors
 * Returns supervisors for a specific active group
 */
export const GET = proxyGet(
  (p) => `/api/active/groups/${requirePathSegmentParam(p)}/supervisors`,
  { raw: true },
);
