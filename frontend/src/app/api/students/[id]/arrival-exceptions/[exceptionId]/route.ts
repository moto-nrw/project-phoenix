import { createDeleteHandler, createPutHandler } from "@/lib/route-wrapper";
import { apiDelete, apiPut } from "@/lib/api-helpers";

// PUT /api/students/[id]/arrival-exceptions/[exceptionId] - Update an arrival exception
export const PUT = createPutHandler(async (_request, body, token, params) => {
  const { id, exceptionId } = params;

  const response = await apiPut(
    `/api/students/${String(id)}/arrival-exceptions/${String(exceptionId)}`,
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type
  return response.data;
});

// DELETE /api/students/[id]/arrival-exceptions/[exceptionId] - Delete an arrival exception
export const DELETE = createDeleteHandler(async (_request, token, params) => {
  const { id, exceptionId } = params;

  await apiDelete(
    `/api/students/${String(id)}/arrival-exceptions/${String(exceptionId)}`,
    token,
  );
  return null;
});
