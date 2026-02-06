import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

/**
 * Request body for starting a break
 */
interface StartBreakRequest {
  planned_duration_minutes?: number;
}

/**
 * POST /api/time-tracking/break/start
 * Start a new break for the current session
 */
export const POST = createPostHandler<unknown, StartBreakRequest>(
  async (
    _request: NextRequest,
    body: StartBreakRequest,
    token: string,
    _params: Record<string, unknown>,
  ) => {
    const response = await apiPost<{ data: unknown }, StartBreakRequest>(
      "/api/time-tracking/break/start",
      token,
      body,
    );
    return response.data;
  },
);
