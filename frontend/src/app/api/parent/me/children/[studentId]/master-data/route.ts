import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy GET /api/parent/me/children/{studentId}/master-data → backend.
 * Returns the structured Stammdaten view (child data + the calling guardian's
 * own contact data + pending change requests).
 */
export const GET = proxyGet<unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/master-data`,
);
