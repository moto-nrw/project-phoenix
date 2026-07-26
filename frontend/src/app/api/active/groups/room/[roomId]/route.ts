// app/api/active/groups/room/[roomId]/route.ts
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Handler for GET /api/active/groups/room/[roomId]
 * Returns active groups in a specific room
 */
export const GET = proxyGet(
  (p) => `/api/active/groups/room/${requirePathSegmentParam(p, "roomId")}`,
  { raw: true },
);
