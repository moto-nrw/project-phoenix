import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

/**
 * POST /api/time-tracking/break/start
 * Start a new break for the current session
 */
export const POST = createPostHandler<unknown>(
  async (_request: NextRequest, _body: unknown, token: string) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/time-tracking/break/start",
      token,
    );
    return response.data;
  },
);
