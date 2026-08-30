import { proxyPut } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface EditOfferingRequestBody {
  offerings: { offering_id: string; selected_days: string[] }[];
  effective_from: string;
  note?: string;
  complete_withdrawal_confirmed?: boolean;
  expected_version: string;
}

/**
 * Proxy PUT /api/parent/me/children/{studentId}/care-offerings/requests/
 * {requestId} → backend. Changes the guardian's own still-open offering change
 * request. The body is the same shape as the create call (`offerings`), so the
 * catalog screen feeds both without renaming a field. 409
 * `change_request_stale` on a version mismatch.
 */
export const PUT = proxyPut<unknown, EditOfferingRequestBody>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/care-offerings/requests/${requirePathSegmentParam(params, "requestId")}`,
);
