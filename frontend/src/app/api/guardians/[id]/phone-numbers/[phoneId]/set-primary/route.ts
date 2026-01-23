import { createPostHandler } from "@/lib/route-wrapper";
import { apiPost } from "@/lib/api-helpers";

// POST /api/guardians/[id]/phone-numbers/[phoneId]/set-primary - Set as primary
export const POST = createPostHandler(async (request, _body, token, params) => {
  const { id, phoneId } = params;
  const guardianId = String(id);
  const phoneIdStr = String(phoneId);

  const response = await apiPost(
    `/api/guardians/${guardianId}/phone-numbers/${phoneIdStr}/set-primary`,
    token,
    {},
  );
  // @ts-expect-error - API helper returns unknown type
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  return response.data;
});
