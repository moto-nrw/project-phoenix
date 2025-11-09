import {
  createGetHandler,
  createPutHandler,
  createDeleteHandler,
} from "@/lib/route-wrapper";
import { apiGet, apiPut, apiDelete } from "@/lib/api-helpers";

// GET /api/guardians/[id] - Get guardian by ID
export const GET = createGetHandler(async (request, token, params) => {
  const { id } = params;
  const guardianId = String(id);

  const response = await apiGet(`/api/guardians/${guardianId}`, token);
  // @ts-expect-error - API helper returns unknown type
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  return response.data;
});

// PUT /api/guardians/[id] - Update guardian
export const PUT = createPutHandler(async (request, body, token, params) => {
  const { id } = params;
  const guardianId = String(id);

  const response = await apiPut(`/api/guardians/${guardianId}`, token, body);
  // @ts-expect-error - API helper returns unknown type
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  return response.data;
});

// DELETE /api/guardians/[id] - Delete guardian
export const DELETE = createDeleteHandler(async (request, token, params) => {
  const { id } = params;
  const guardianId = String(id);

  await apiDelete(`/api/guardians/${guardianId}`, token);
  return null;
});
