import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import { proxyPost } from "~/lib/route-proxy.server";

interface BackendEnvelope<T> {
  data: T;
}

/**
 * Proxy GET /api/students/change-requests → backend (#2432).
 * Aggregierte Eltern-Anfragenliste über die vier Anfragearten, offen oder
 * Historie, cursor-paginiert. Nur die allowgelisteten Parameter werden
 * weitergereicht.
 */
const FORWARDED_PARAMS = [
  "view",
  "search",
  "student_id",
  "types",
  "status",
  "from",
  "to",
  "cursor",
  "limit",
] as const;

export const GET = createGetHandler<unknown>(async (request, token) => {
  const incoming = new URL(request.url).searchParams;
  const params = new URLSearchParams();
  for (const name of FORWARDED_PARAMS) {
    const value = incoming.get(name);
    if (value) params.set(name, value);
  }
  const query = params.size > 0 ? `?${params.toString()}` : "";
  const response = await apiGet<BackendEnvelope<unknown>>(
    `/api/students/change-requests${query}`,
    token,
  );
  return response.data;
});

export const POST = proxyPost("/api/students/change-requests/bulk-approve");
