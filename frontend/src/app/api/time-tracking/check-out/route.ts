import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

/**
 * POST /api/time-tracking/check-out
 * Check out from work
 */
export const POST = createPostHandler<unknown, never>(
  async (_request: NextRequest, _body: never, token: string) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/time-tracking/check-out",
      token,
    );
    return response.data;
  },
);
