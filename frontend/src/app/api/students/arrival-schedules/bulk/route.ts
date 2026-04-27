import { createPostHandler } from "@/lib/route-wrapper";
import { apiPost } from "@/lib/api-helpers";

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
