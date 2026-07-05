// app/api/active/groups/[id]/route.ts
import { proxyDelete, proxyGet, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Type definition for group update request
 */
interface GroupUpdateRequest {
  name?: string;
  description?: string;
  room_id?: string;
}

const ENDPOINT = "/api/active/groups";

export const GET = proxyGet(
  (p) => `${ENDPOINT}/${requirePathSegmentParam(p)}`,
  {
    raw: true,
  },
);
export const PUT = proxyPut<unknown, GroupUpdateRequest>(
  (p) => `${ENDPOINT}/${requirePathSegmentParam(p)}`,
  { raw: true },
);
export const DELETE = proxyDelete(
  (p) => `${ENDPOINT}/${requirePathSegmentParam(p)}`,
);
