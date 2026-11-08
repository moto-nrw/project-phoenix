import { createDeleteHandler } from "@/lib/route-wrapper.server";
import { apiDelete } from "@/lib/api-helpers.server";
import { proxyGet, proxyPut } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// GET /api/guardians/[id] - Get guardian by ID
export const GET = proxyGet(
  (p) => `/api/guardians/${requirePathSegmentParam(p)}`,
);

// PUT /api/guardians/[id] - Update guardian
export const PUT = proxyPut(
  (p) => `/api/guardians/${requirePathSegmentParam(p)}`,
);

// DELETE /api/guardians/[id] - Delete guardian
// Forwards the optional `force` query param: without it the backend refuses a
// guardian still linked to students (409); with `force=true` it performs the
// deliberate full delete (admin-only, enforced in the backend handler).
export const DELETE = createDeleteHandler(async (request, token, params) => {
  const { id } = params;
  const guardianId = String(id);

  const queryString = request.nextUrl.searchParams.toString();
  const endpoint = queryString
    ? `/api/guardians/${encodeURIComponent(guardianId)}?${queryString}`
    : `/api/guardians/${encodeURIComponent(guardianId)}`;

  await apiDelete(endpoint, token);
  return null;
});
