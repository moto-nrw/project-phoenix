import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// GET /api/parent/me/children/{studentId}/pickup-permission-requests
// The child's pickup permission requests the account may see (#4780).
export const GET = proxyGet<unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/pickup-permission-requests`,
);
