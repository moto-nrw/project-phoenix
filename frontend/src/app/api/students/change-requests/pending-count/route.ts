import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface BackendEnvelope<T> {
  data: T;
}

/**
 * Proxy GET /api/students/change-requests/pending-count → backend. Returns the
 * combined number of pending parent change requests (master-data + care
 * schedule) for the tenant, driving the Änderungsanfragen sidebar badge.
 */
export const GET = createGetHandler<{ pending_count: number }>(
  async (_request, token) => {
    const response = await apiGet<BackendEnvelope<{ pending_count: number }>>(
      "/api/students/change-requests/pending-count",
      token,
    );
    return response.data;
  },
);
