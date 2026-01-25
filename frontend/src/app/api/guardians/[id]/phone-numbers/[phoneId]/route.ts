import { createPutHandler, createDeleteHandler } from "@/lib/route-wrapper";
import { apiPut, apiDelete } from "@/lib/api-helpers";

// PUT /api/guardians/[id]/phone-numbers/[phoneId] - Update phone number
export const PUT = createPutHandler(async (request, body, token, params) => {
  const { id, phoneId } = params;
  const guardianId = String(id);
  const phoneIdStr = String(phoneId);

  const response = await apiPut(
    `/api/guardians/${guardianId}/phone-numbers/${phoneIdStr}`,
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  return response.data;
});

// DELETE /api/guardians/[id]/phone-numbers/[phoneId] - Delete phone number
export const DELETE = createDeleteHandler(async (request, token, params) => {
  const { id, phoneId } = params;
  const guardianId = String(id);
  const phoneIdStr = String(phoneId);

  await apiDelete(
    `/api/guardians/${guardianId}/phone-numbers/${phoneIdStr}`,
    token,
  );
  return null;
});
