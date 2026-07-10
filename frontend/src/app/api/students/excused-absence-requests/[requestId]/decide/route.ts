import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

interface DecideBody {
  approve: boolean;
  reason?: string;
}

interface BackendEnvelope<T> {
  data: T;
}

/**
 * Proxy POST /api/students/excused-absence-requests/{requestId}/decide →
 * backend. Approves (marks the child excused for the requested days) or rejects
 * (reason required) one parent "entschuldigt" absence request. A 409 with code
 * `change_request_not_pending` means the request was already decided/withdrawn;
 * `guardian_access_revoked` means the submitting guardian lost access to the
 * child before approval (staff should reject it instead).
 */
export const POST = createPostHandler<unknown, DecideBody>(
  async (_request, body, token, params) => {
    const requestId = String(params.requestId);
    const response = await apiPost<BackendEnvelope<unknown>, DecideBody>(
      `/api/students/excused-absence-requests/${encodeURIComponent(requestId)}/decide`,
      token,
      body,
    );
    return response.data;
  },
);
