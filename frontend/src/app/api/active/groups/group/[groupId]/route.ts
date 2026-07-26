// app/api/active/groups/group/[groupId]/route.ts
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Handler for GET /api/active/groups/group/[groupId]
 * Returns active groups for a specific education group
 */
export const GET = proxyGet(
  (p) => `/api/active/groups/group/${requirePathSegmentParam(p, "groupId")}`,
  { raw: true },
);
