import { createPostHandler } from "@/lib/route-wrapper";
import { apiPost } from "@/lib/api-helpers";

// POST /api/students/[id]/pickup-exceptions - Create a pickup exception
export const POST = createPostHandler(async (_request, body, token, params) => {
  const { id } = params;

  const response = await apiPost(
    `/api/students/${String(id)}/pickup-exceptions`,
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  return response.data;
});
