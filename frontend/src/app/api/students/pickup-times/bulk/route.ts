import { createPostHandler } from "@/lib/route-wrapper";
import { apiPost } from "@/lib/api-helpers";

// POST /api/students/pickup-times/bulk - Get effective pickup times for multiple students
export const POST = createPostHandler(async (_request, body, token) => {
  const response = await apiPost(
    "/api/students/pickup-times/bulk",
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  return response.data;
});
