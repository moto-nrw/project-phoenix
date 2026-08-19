import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

interface DecideBody {
  approve: boolean;
  reason?: string;
  /** Rule-added offerings staff unticked for this one approval (#2370). */
  excluded_offering_ids?: string[];
}

interface BackendEnvelope<T> {
  data: T;
}

/**
 * Proxy POST /api/students/offering-change-requests/{requestId}/decide →
 * backend. Approves (applies the dated switch to the child's offerings) or
 * rejects (reason required) one request (#1665).
 */
export const POST = createPostHandler<unknown, DecideBody>(
  async (_request, body, token, params) => {
    const requestId = String(params.requestId);
    const response = await apiPost<BackendEnvelope<unknown>, DecideBody>(
      `/api/students/offering-change-requests/${encodeURIComponent(requestId)}/decide`,
      token,
      body,
    );
    return response.data;
  },
);
