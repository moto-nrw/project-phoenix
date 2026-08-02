import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy POST /api/parent/me/children/{studentId}/care-offerings/requests →
 * backend. Submits a post-enrollment offering change request; the backend
 * verifies guardianship, the school's gate and the selection rules, and returns
 * the refreshed view (now carrying the pending request).
 */
export const POST = proxyPost<unknown, unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/care-offerings/requests`,
);
