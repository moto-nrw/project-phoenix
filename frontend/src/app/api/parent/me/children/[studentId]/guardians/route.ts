import {
  createParentGetHandler,
  parentApiGet,
} from "~/lib/parent/route-wrapper.server";

// GET /api/parent/me/children/{studentId}/guardians
// Lists the child's guardians with contact + pickup detail. Ownership is
// enforced server-side against the calling account's guardian links.
export const GET = createParentGetHandler<unknown>(
  async (_request, token, params) => {
    const studentId = String(params.studentId);
    return parentApiGet<unknown>(
      `/parent/me/children/${encodeURIComponent(studentId)}/guardians`,
      token,
    );
  },
);
