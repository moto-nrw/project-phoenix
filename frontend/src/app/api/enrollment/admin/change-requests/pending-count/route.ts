import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface BackendEnvelope<T> {
  data: T;
}

/**
 * Proxy GET /api/enrollment/admin/change-requests/pending-count → backend
 * (#2435). Zahl der offenen Anmeldungsänderungen für das Badge am
 * Anfragen-Eintrag.
 */
export const GET = createGetHandler<unknown>(async (_request, token) => {
  const response = await apiGet<BackendEnvelope<unknown>>(
    "/api/enrollment/admin/change-requests/pending-count",
    token,
  );
  return response.data;
});
