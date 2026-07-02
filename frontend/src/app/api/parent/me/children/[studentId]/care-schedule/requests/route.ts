import {
  createParentPostHandler,
  parentApiPost,
} from "~/lib/parent/route-wrapper.server";

/**
 * Proxy POST /api/parent/me/children/{studentId}/care-schedule/requests →
 * backend. Submits a permanent weekly-plan change request; the backend verifies
 * guardianship + the feature/permission gate and returns the refreshed
 * care-schedule view (now carrying the pending request).
 */
export const POST = createParentPostHandler<unknown, unknown>(
  async (_request, body, token, params) => {
    const studentId = String(params.studentId);
    return parentApiPost<unknown, unknown>(
      `/parent/me/children/${encodeURIComponent(studentId)}/care-schedule/requests`,
      token,
      body,
    );
  },
);
