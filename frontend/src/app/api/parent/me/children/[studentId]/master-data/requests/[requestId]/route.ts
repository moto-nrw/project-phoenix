import { proxyPut } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface EditMasterDataRequestBody {
  new_value: unknown;
  expected_version: string;
}

/**
 * Proxy PUT /api/parent/me/children/{studentId}/master-data/requests/
 * {requestId} → backend. Changes the guardian's own still-pending Stammdaten
 * change request; 409 `change_request_stale` on a version mismatch.
 */
export const PUT = proxyPut<unknown, EditMasterDataRequestBody>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/master-data/requests/${requirePathSegmentParam(params, "requestId")}`,
);
