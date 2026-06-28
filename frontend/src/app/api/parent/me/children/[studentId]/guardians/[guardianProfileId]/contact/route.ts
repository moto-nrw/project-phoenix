import { proxyPut } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// PUT /api/parent/me/children/{studentId}/guardians/{guardianProfileId}/contact
// Edits a contact-only guardian's contact data. Permission (guardian.edit) +
// the contact-only / self constraint are enforced server-side.
export const PUT = proxyPut<unknown, unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/guardians/${requirePathSegmentParam(params, "guardianProfileId")}/contact`,
);
