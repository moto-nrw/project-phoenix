import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface BackendEnvelope<T> {
  data: T;
}

/**
 * Proxy GET /api/students/care-schedule-change-requests/{requestId} →
 * backend (#3135). One care-schedule request of any status, read from a
 * message-thread pill. The backend gates on users:update and the per-child
 * review scope; 403/404 pass through unchanged.
 */
export const GET = createGetHandler<unknown>(
  async (_request, token, params) => {
    const requestId = String(params.requestId);
    const response = await apiGet<BackendEnvelope<unknown>>(
      `/api/students/care-schedule-change-requests/${encodeURIComponent(requestId)}`,
      token,
    );
    return response.data;
  },
);
