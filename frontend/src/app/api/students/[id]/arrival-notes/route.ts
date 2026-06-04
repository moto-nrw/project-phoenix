import { createPostHandler } from "@/lib/route-wrapper.server";
import { apiPost } from "@/lib/api-helpers.server";

// POST /api/students/[id]/arrival-notes - Create an arrival note
export const POST = createPostHandler(async (_request, body, token, params) => {
  const { id } = params;

  const response = await apiPost(
    `/api/students/${String(id)}/arrival-notes`,
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type
  return response.data;
});
