import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy POST /api/parent/me/messages/children/{studentId}/requests/{requestId}/withdraw
 * → backend. Withdraws a pending guardian request.
 */
export const POST = proxyPost<unknown, Record<string, never>>(
  (params) =>
    `/parent/me/messages/children/${requirePathSegmentParam(params, "studentId")}/requests/${requirePathSegmentParam(params, "requestId")}/withdraw`,
);
