import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface BackendEnvelope<T> {
  data: T;
}

/**
 * Proxy GET /api/students/care-schedule-change-requests/history → backend.
 * Returns the tenant's decided care-schedule change requests, cursor-paginated.
 * Only the allowlisted params cursor + limit are forwarded.
 */
export const GET = createGetHandler<unknown>(async (request, token) => {
  const incoming = new URL(request.url).searchParams;
  const params = new URLSearchParams();
  const cursor = incoming.get("cursor");
  if (cursor) params.set("cursor", cursor);
  const limit = incoming.get("limit");
  if (limit) params.set("limit", limit);
  const query = params.size > 0 ? `?${params.toString()}` : "";
  const response = await apiGet<BackendEnvelope<unknown>>(
    `/api/students/care-schedule-change-requests/history${query}`,
    token,
  );
  return response.data;
});
