import {
  createParentPutHandler,
  parentApiPut,
} from "~/lib/parent/route-wrapper.server";

// PUT /api/parent/me/children/{studentId}/guardians/{guardianProfileId}/contact
// Edits a contact-only guardian's contact data. Permission (guardian.edit) +
// the contact-only / self constraint are enforced server-side.
export const PUT = createParentPutHandler<unknown, unknown>(
  async (_request, body, token, params) => {
    const studentId = String(params.studentId);
    const guardianProfileId = String(params.guardianProfileId);
    return parentApiPut<unknown, unknown>(
      `/parent/me/children/${encodeURIComponent(studentId)}/guardians/${encodeURIComponent(guardianProfileId)}/contact`,
      token,
      body,
    );
  },
);
