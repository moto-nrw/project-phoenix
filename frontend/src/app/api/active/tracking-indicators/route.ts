import { createPostHandler } from "@/lib/route-wrapper.server";
import { apiPost } from "@/lib/api-helpers.server";
import type { ApiResponse } from "@/lib/api-helpers.server";
import type { TrackingIndicatorsResponse } from "@/lib/active-helpers";

// POST /api/active/tracking-indicators - Bulk check if students visited configured rooms/activities today
export const POST = createPostHandler(async (_request, body, token) => {
  const response = await apiPost<ApiResponse<TrackingIndicatorsResponse>>(
    "/api/active/tracking-indicators",
    token,
    body,
  );
  return response.data;
});
