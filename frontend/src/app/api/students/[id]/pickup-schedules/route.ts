import { createGetHandler, createPutHandler } from "@/lib/route-wrapper";
import { apiGet, apiPut } from "@/lib/api-helpers";

// GET /api/students/[id]/pickup-schedules - Get pickup schedules and exceptions for a student
export const GET = createGetHandler(async (_request, token, params) => {
  const { id } = params;

  const response = await apiGet(
    `/api/students/${String(id)}/pickup-schedules`,
    token,
  );
  // @ts-expect-error - API helper returns unknown type
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  return response.data;
});

// PUT /api/students/[id]/pickup-schedules - Bulk update weekly pickup schedules
export const PUT = createPutHandler(async (_request, body, token, params) => {
  const { id } = params;

  const response = await apiPut(
    `/api/students/${String(id)}/pickup-schedules`,
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  return response.data;
});
