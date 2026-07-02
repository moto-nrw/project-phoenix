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
 * Proxy POST /api/students/care-schedule-change-requests/{requestId}/decide →
 * backend. Approves (applies the weekly plan) or rejects (reason required) one
 * parent care-schedule change request (#1803).
 */
export const POST = createPostHandler<unknown, DecideBody>(
  async (_request, body, token, params) => {
    const requestId = String(params.requestId);
    const response = await apiPost<BackendEnvelope<unknown>, DecideBody>(
      `/api/students/care-schedule-change-requests/${encodeURIComponent(requestId)}/decide`,
      token,
      body,
    );
    return response.data;
  },
);
