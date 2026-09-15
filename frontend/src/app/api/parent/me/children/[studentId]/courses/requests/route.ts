import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy POST /api/parent/me/children/{studentId}/courses/requests → backend.
 * Asks the OGS for one course; the backend checks guardianship, the school's
 * gate and the capacity, and returns the refreshed course list (#3075).
 */
export const POST = proxyPost<unknown, unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/courses/requests`,
);
