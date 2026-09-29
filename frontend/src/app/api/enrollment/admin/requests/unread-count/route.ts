import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface BackendEnvelope<T> {
  data: T;
}

/**
 * Proxy GET /api/enrollment/admin/requests/unread-count → backend (#3778).
 * Zahl der ungelesenen Anmeldungen der angemeldeten Person für das Badge am
 * Bereich „Anmeldungen".
 */
export const GET = createGetHandler<unknown>(async (_request, token) => {
  const response = await apiGet<BackendEnvelope<unknown>>(
    "/api/enrollment/admin/requests/unread-count",
    token,
  );
  return response.data;
});
