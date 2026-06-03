import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * GET /api/time-tracking/current
 * Get current active work session
 */
export const GET = createGetHandler<unknown>(
  async (_request: NextRequest, token: string) => {
    const response = await apiGet<{ data: unknown }>(
      "/api/time-tracking/current",
      token,
    );
    return response.data;
  },
);
