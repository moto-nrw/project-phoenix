import { createGetHandler, createPutHandler } from "@/lib/route-wrapper.server";
import { apiGet, apiPut } from "@/lib/api-helpers.server";

// GET /api/students/[id]/arrival-schedules - Fetch arrival schedules, exceptions and notes
export const GET = createGetHandler(async (_request, token, params) => {
  const { id } = params;

  const response = await apiGet(
    `/api/students/${String(id)}/arrival-schedules`,
    token,
  );
  // @ts-expect-error - API helper returns unknown type
  return response.data;
});

// PUT /api/students/[id]/arrival-schedules - Bulk update weekly arrival schedules
export const PUT = createPutHandler(async (_request, body, token, params) => {
  const { id } = params;

  const response = await apiPut(
    `/api/students/${String(id)}/arrival-schedules`,
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type
  return response.data;
});
