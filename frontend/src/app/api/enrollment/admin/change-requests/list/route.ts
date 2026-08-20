import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface BackendEnvelope<T> {
  data: T;
}

/**
 * Proxy GET /api/enrollment/admin/change-requests/list → backend (#2435).
 * Anmeldungsänderungen im gemeinsamen Anzeigeformat des Anfragen-Moduls,
 * offen oder Historie, cursor-paginiert. Nur die allowgelisteten Parameter
 * werden weitergereicht.
 */
const FORWARDED_PARAMS = [
  "view",
  "search",
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
    `/api/enrollment/admin/change-requests/list${query}`,
    token,
  );
  return response.data;
});
