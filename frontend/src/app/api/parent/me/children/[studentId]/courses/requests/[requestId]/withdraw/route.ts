import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy POST .../courses/requests/{requestId}/withdraw → backend. Takes back
 * the caller's own open course request (#3075).
 */
export const POST = proxyPost<unknown, unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/courses/requests/${requirePathSegmentParam(params, "requestId")}/withdraw`,
);
