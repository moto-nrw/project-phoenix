import { createPostHandler } from "@/lib/route-wrapper.server";
import { apiPost } from "@/lib/api-helpers.server";

// POST /api/students/pickup-times/bulk - Get effective pickup times for multiple students
export const POST = createPostHandler(async (_request, body, token) => {
  const response = await apiPost(
    "/api/students/pickup-times/bulk",
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type

  return response.data;
});
