import { proxyPut } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// PUT /api/parent/me/children/{studentId}/guardians/{guardianProfileId}/pickup
// Edits the per-child pickup/relationship fields. The can_pickup /
// is_emergency_contact flags require pickup.manage; note/priority require
// guardian.edit. Enforced server-side.
export const PUT = proxyPut<unknown, unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/guardians/${requirePathSegmentParam(params, "guardianProfileId")}/pickup`,
);
