import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/students/master-data-change-requests → backend.
 * Returns the tenant's pending parent Stammdaten change requests.
 */
export const GET = createGetHandler<unknown>(async (_request, token) => {
  return apiGet<unknown>("/api/students/master-data-change-requests", token);
});
