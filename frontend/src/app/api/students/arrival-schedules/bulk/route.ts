import { createPostHandler } from "@/lib/route-wrapper.server";
import { apiPost } from "@/lib/api-helpers.server";

// POST /api/students/arrival-schedules/bulk - Bulk upsert arrival schedules for a school class
export const POST = createPostHandler(async (_request, body, token) => {
  const response = await apiPost(
    "/api/students/arrival-schedules/bulk",
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type
  return response.data;
});
