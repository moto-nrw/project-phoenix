import {
  createParentDeleteHandler,
  parentApiDelete,
} from "~/lib/parent/route-wrapper.server";

// DELETE /api/parent/me/children/{studentId}/related-accounts/{guardianProfileId}
// Remove another account's access to the child. The remove gate + the
// primary-guardian protection are enforced server-side.
export const DELETE = createParentDeleteHandler<unknown>(
  async (_request, token, params) => {
    const studentId = String(params.studentId);
    const guardianProfileId = String(params.guardianProfileId);
    return parentApiDelete<unknown>(
      `/parent/me/children/${encodeURIComponent(studentId)}/related-accounts/${encodeURIComponent(guardianProfileId)}`,
      token,
    );
  },
);
