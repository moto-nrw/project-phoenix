import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface BackendEnvelope<T> {
  data: T;
}

/**
 * Proxy GET /api/students/care-schedule-change-requests → backend.
 * Returns the tenant's pending parent care-schedule change requests (#1803).
 */
export const GET = createGetHandler<unknown>(async (_request, token) => {
  const response = await apiGet<BackendEnvelope<unknown>>(
    "/api/students/care-schedule-change-requests",
    token,
  );
  return response.data;
});
