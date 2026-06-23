import {
  createParentPutHandler,
  parentApiPut,
} from "~/lib/parent/route-wrapper.server";

// PUT /api/parent/me/children/{studentId}/guardians/{guardianProfileId}/pickup
// Edits the per-child pickup/relationship fields. The can_pickup /
// is_emergency_contact flags require pickup.manage; note/priority require
// guardian.edit. Enforced server-side.
export const PUT = createParentPutHandler<unknown, unknown>(
  async (_request, body, token, params) => {
    const studentId = String(params.studentId);
    const guardianProfileId = String(params.guardianProfileId);
    return parentApiPut<unknown, unknown>(
      `/parent/me/children/${encodeURIComponent(studentId)}/guardians/${encodeURIComponent(guardianProfileId)}/pickup`,
      token,
      body,
    );
  },
);
