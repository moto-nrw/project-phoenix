import { createPutHandler, createDeleteHandler } from "@/lib/route-wrapper";
import { apiPut, apiDelete } from "@/lib/api-helpers";

// PUT /api/students/[id]/pickup-exceptions/[exceptionId] - Update a pickup exception
export const PUT = createPutHandler(async (_request, body, token, params) => {
  const { id, exceptionId } = params;

  const response = await apiPut(
    `/api/students/${String(id)}/pickup-exceptions/${String(exceptionId)}`,
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  return response.data;
});

// DELETE /api/students/[id]/pickup-exceptions/[exceptionId] - Delete a pickup exception
export const DELETE = createDeleteHandler(async (_request, token, params) => {
  const { id, exceptionId } = params;

  await apiDelete(
    `/api/students/${String(id)}/pickup-exceptions/${String(exceptionId)}`,
    token,
  );
  return null;
});
