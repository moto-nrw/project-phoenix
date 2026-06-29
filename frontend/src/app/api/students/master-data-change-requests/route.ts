import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface BackendEnvelope<T> {
  data: T;
}

/**
 * Proxy GET /api/students/master-data-change-requests → backend.
 * Returns the tenant's pending parent Stammdaten change requests.
 */
export const GET = createGetHandler<unknown>(async (_request, token) => {
  const response = await apiGet<BackendEnvelope<unknown>>(
    "/api/students/master-data-change-requests",
    token,
  );
  return response.data;
});
