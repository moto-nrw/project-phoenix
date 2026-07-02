import {
  createParentGetHandler,
  parentApiGet,
} from "~/lib/parent/route-wrapper.server";

/**
 * Proxy GET /api/parent/me/children/{studentId}/care-schedule → backend.
 * Returns the child's standard weekly care plan (Mon-Fr arrival/pickup/modes),
 * the guardian's own still-open change request (if any), and whether the
 * guardian may submit a new request.
 */
export const GET = createParentGetHandler<unknown>(
  async (_request, token, params) => {
    const studentId = String(params.studentId);
    return parentApiGet<unknown>(
      `/parent/me/children/${encodeURIComponent(studentId)}/care-schedule`,
      token,
    );
  },
);
