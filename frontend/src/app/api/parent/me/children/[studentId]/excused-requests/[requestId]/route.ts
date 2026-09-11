import { proxyPut } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface EditExcusedRequestBody {
  dates: string[];
  note: string;
  expected_version: string;
}

/**
 * Proxy PUT /api/parent/me/children/{studentId}/excused-requests/{requestId}
 * → backend. Changes the guardian's own still-pending sick or excused absence
 * request. The route retains its legacy excused-only name. The backend answers
 * 409 `change_request_stale` when `expected_version` no longer matches, and
 * verifies guardianship from the JWT.
 */
export const PUT = proxyPut<unknown, EditExcusedRequestBody>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/excused-requests/${requirePathSegmentParam(params, "requestId")}`,
);
