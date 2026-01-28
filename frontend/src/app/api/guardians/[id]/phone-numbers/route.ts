import { createGetHandler, createPostHandler } from "@/lib/route-wrapper";
import { apiGet, apiPost } from "@/lib/api-helpers";

// GET /api/guardians/[id]/phone-numbers - List guardian phone numbers
export const GET = createGetHandler(async (request, token, params) => {
  const { id } = params;
  const guardianId = String(id);

  const response = await apiGet(
    `/api/guardians/${guardianId}/phone-numbers`,
    token,
  );
  // @ts-expect-error - API helper returns unknown type
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  return response.data;
});

// POST /api/guardians/[id]/phone-numbers - Add phone number
export const POST = createPostHandler(async (request, body, token, params) => {
  const { id } = params;
  const guardianId = String(id);

  const response = await apiPost(
    `/api/guardians/${guardianId}/phone-numbers`,
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  return response.data;
});
