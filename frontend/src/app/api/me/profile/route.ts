import { createGetHandler, createPutHandler } from "~/lib/route-wrapper";
import { apiGet, apiPut } from "~/lib/api-helpers";
import type { ApiResponse } from "~/lib/api-helpers";

export const GET = createGetHandler(async (request, token, _params) => {
  const response = await apiGet<ApiResponse<unknown>>(`/api/me/profile`, token);
  return response.data;
});

export const PUT = createPutHandler<unknown, Record<string, unknown>>(async (request, body, token, _params) => {
  const response = await apiPut<ApiResponse<unknown>, Record<string, unknown>>('/api/me/profile', token, body);
  return response.data;
});