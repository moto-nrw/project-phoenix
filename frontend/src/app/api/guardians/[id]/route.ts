import {
  createGetHandler,
  createPutHandler,
  createDeleteHandler,
} from "@/lib/route-wrapper.server";
import { apiGet, apiPut, apiDelete } from "@/lib/api-helpers.server";

// GET /api/guardians/[id] - Get guardian by ID
export const GET = createGetHandler(async (request, token, params) => {
  const { id } = params;
  const guardianId = String(id);

  const response = await apiGet(`/api/guardians/${guardianId}`, token);
  // @ts-expect-error - API helper returns unknown type

  return response.data;
});

// PUT /api/guardians/[id] - Update guardian
export const PUT = createPutHandler(async (request, body, token, params) => {
  const { id } = params;
  const guardianId = String(id);

  const response = await apiPut(`/api/guardians/${guardianId}`, token, body);
  // @ts-expect-error - API helper returns unknown type

  return response.data;
});

// DELETE /api/guardians/[id] - Delete guardian
// Forwards the optional `force` query param: without it the backend refuses a
// guardian still linked to students (409); with `force=true` it performs the
// deliberate full delete (admin-only, enforced in the backend handler).
export const DELETE = createDeleteHandler(async (request, token, params) => {
  const { id } = params;
  const guardianId = String(id);

  const queryString = request.nextUrl.searchParams.toString();
  const endpoint = queryString
    ? `/api/guardians/${guardianId}?${queryString}`
    : `/api/guardians/${guardianId}`;

  await apiDelete(endpoint, token);
  return null;
});
