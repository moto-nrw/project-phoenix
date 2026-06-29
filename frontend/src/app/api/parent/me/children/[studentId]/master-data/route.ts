import {
  createParentGetHandler,
  parentApiGet,
} from "~/lib/parent/route-wrapper.server";

/**
 * Proxy GET /api/parent/me/children/{studentId}/master-data → backend.
 * Returns the structured Stammdaten view (child data + the calling guardian's
 * own contact data + pending change requests).
 */
export const GET = createParentGetHandler<unknown>(
  async (_request, token, params) => {
    const studentId = String(params.studentId);
    return parentApiGet<unknown>(
      `/parent/me/children/${encodeURIComponent(studentId)}/master-data`,
      token,
    );
  },
);
