import { createPostHandler } from "@/lib/route-wrapper";
import { apiPost } from "@/lib/api-helpers";

// POST /api/students/arrival-times/bulk - Get effective arrival times for multiple students
export const POST = createPostHandler(async (_request, body, token) => {
  const response = await apiPost(
    "/api/students/arrival-times/bulk",
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type

  return response.data;
});
