// app/api/active/visits/[id]/route.ts
import { proxyDelete, proxyGet, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Type definition for visit update request
 */
interface VisitUpdateRequest {
  student_id?: string;
  active_group_id?: string;
  start_time?: string;
  end_time?: string;
}

const ENDPOINT = "/api/active/visits";

export const GET = proxyGet(
  (p) => `${ENDPOINT}/${requirePathSegmentParam(p)}`,
  {
    raw: true,
  },
);
export const PUT = proxyPut<unknown, VisitUpdateRequest>(
  (p) => `${ENDPOINT}/${requirePathSegmentParam(p)}`,
  { raw: true },
);
export const DELETE = proxyDelete(
  (p) => `${ENDPOINT}/${requirePathSegmentParam(p)}`,
);
