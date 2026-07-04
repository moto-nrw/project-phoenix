import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy POST /api/parent/me/children/{studentId}/care-schedule/requests →
 * backend. Submits a permanent weekly-plan change request; the backend verifies
 * guardianship + the feature/permission gate and returns the refreshed
 * care-schedule view (now carrying the pending request).
 */
export const POST = proxyPost<unknown, unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/care-schedule/requests`,
);
