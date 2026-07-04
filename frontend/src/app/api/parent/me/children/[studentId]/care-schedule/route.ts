import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy GET /api/parent/me/children/{studentId}/care-schedule → backend.
 * Returns the child's standard weekly care plan (Mon-Fr arrival/pickup/modes),
 * the guardian's own still-open change request (if any), and whether the
 * guardian may submit a new request.
 */
export const GET = proxyGet<unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/care-schedule`,
);
