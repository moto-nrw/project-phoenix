import {
  createParentPostHandler,
  parentApiPost,
} from "~/lib/parent/route-wrapper.server";

/**
 * Proxy POST
 * /api/parent/me/children/{studentId}/care-schedule/requests/{requestId}/withdraw
 * → backend. Withdraws the guardian's own still-open change request and returns
 * the refreshed care-schedule view (pending request cleared).
 */
export const POST = createParentPostHandler<unknown, unknown>(
  async (_request, body, token, params) => {
    const studentId = String(params.studentId);
    const requestId = String(params.requestId);
    return parentApiPost<unknown, unknown>(
      `/parent/me/children/${encodeURIComponent(studentId)}/care-schedule/requests/${encodeURIComponent(requestId)}/withdraw`,
      token,
      body,
    );
  },
);
