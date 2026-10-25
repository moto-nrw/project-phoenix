import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// POST /api/parent/me/children/{studentId}/guardians/{guardianProfileId}/pickup-permission-requests
// Asks the OGS to let a contact person collect the child on every day or on
// some weekdays, signed (#4780). parent_portal.pickup.manage is enforced
// server-side.
export const POST = proxyPost<unknown, unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/guardians/${requirePathSegmentParam(params, "guardianProfileId")}/pickup-permission-requests`,
);
