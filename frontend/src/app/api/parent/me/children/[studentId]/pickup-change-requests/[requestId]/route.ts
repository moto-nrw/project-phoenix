import { proxyPut } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface EditPickupChangeRequestBody {
  date: string;
  pickup_time: string;
  reason: string;
  expected_version: string;
}

/**
 * Proxy PUT /api/parent/me/children/{studentId}/pickup-change-requests/
 * {requestId} → backend. Changes the guardian's own still-pending pickup-time
 * request; 409 `change_request_stale` on a version mismatch.
 */
export const PUT = proxyPut<unknown, EditPickupChangeRequestBody>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/pickup-change-requests/${requirePathSegmentParam(params, "requestId")}`,
);
