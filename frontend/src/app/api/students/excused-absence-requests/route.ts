import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface BackendEnvelope<T> {
  data: T;
}

/**
 * Proxy GET /api/students/excused-absence-requests → backend.
 * Returns the tenant's pending parent "entschuldigt" absence requests that
 * await an OGS confirmation (operations.parent_excused_requires_approval).
 */
export const GET = createGetHandler<unknown>(async (_request, token) => {
  const response = await apiGet<BackendEnvelope<unknown>>(
    "/api/students/excused-absence-requests",
    token,
  );
  return response.data;
});
