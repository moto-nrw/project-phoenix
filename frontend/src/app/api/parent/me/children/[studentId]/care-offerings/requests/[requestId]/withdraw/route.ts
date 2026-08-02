import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy POST
 * /api/parent/me/children/{studentId}/care-offerings/requests/{requestId}/withdraw
 * → backend. Withdraws the guardian's own still-open offering change request.
 */
export const POST = proxyPost<unknown, unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/care-offerings/requests/${requirePathSegmentParam(params, "requestId")}/withdraw`,
);
