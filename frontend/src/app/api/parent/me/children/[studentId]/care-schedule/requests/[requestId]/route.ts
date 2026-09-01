import { proxyPut } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface EditCareScheduleRequestBody {
  payload: unknown;
  expected_version: string;
}

/**
 * Proxy PUT /api/parent/me/children/{studentId}/care-schedule/requests/
 * {requestId} → backend. Changes the guardian's own still-pending weekly-plan
 * request; 409 `change_request_stale` on a version mismatch.
 */
export const PUT = proxyPut<unknown, EditCareScheduleRequestBody>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/care-schedule/requests/${requirePathSegmentParam(params, "requestId")}`,
);
