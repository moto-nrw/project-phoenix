import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface BackendEnvelope<T> {
  data: T;
}

/**
 * Proxy GET /api/students/offering-change-requests → backend.
 * Returns the tenant's pending post-enrollment offering change requests
 * (#1665), soonest effective date first.
 */
export const GET = createGetHandler<unknown>(async (_request, token) => {
  const response = await apiGet<BackendEnvelope<unknown>>(
    "/api/students/offering-change-requests",
    token,
  );
  return response.data;
});
