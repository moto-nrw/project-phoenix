import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// POST /api/parent/me/children/{studentId}/pickup-signature-statement
// Loads the canonical declaration before the parent signs it. The backend
// verifies parent_portal.pickup.manage and rebuilds the wording on submission.
export const POST = proxyPost<unknown, unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/pickup-signature-statement`,
);
