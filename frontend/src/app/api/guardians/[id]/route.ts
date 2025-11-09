import { NextRequest } from "next/server";
import { createGetHandler, createPutHandler, createDeleteHandler } from "@/lib/route-wrapper";
import { apiGet, apiPut, apiDelete } from "@/lib/api-helpers";

// GET /api/guardians/[id] - Get guardian by ID
export const GET = createGetHandler(async (request, token, params) => {
  const { id } = await params;

  const response = await apiGet(`/api/guardians/${id}`, token);
  return response.data;
});

// PUT /api/guardians/[id] - Update guardian
export const PUT = createPutHandler(async (request, body, token, params) => {
  const { id } = params;

  const response = await apiPut(`/api/guardians/${id}`, token, body);
  return response.data;
});

// DELETE /api/guardians/[id] - Delete guardian
export const DELETE = createDeleteHandler(async (request, token, params) => {
  const { id } = await params;

  await apiDelete(`/api/guardians/${id}`, token);
  return null;
});
