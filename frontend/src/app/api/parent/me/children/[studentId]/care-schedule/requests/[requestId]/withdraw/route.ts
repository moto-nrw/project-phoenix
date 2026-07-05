import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy POST
 * /api/parent/me/children/{studentId}/care-schedule/requests/{requestId}/withdraw
 * → backend. Withdraws the guardian's own still-open change request and returns
 * the refreshed care-schedule view (pending request cleared).
 */
export const POST = proxyPost<unknown, unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/care-schedule/requests/${requirePathSegmentParam(params, "requestId")}/withdraw`,
);
