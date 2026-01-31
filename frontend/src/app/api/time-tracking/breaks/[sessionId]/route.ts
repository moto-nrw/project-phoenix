import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler, isStringParam } from "~/lib/route-wrapper";

/**
 * GET /api/time-tracking/breaks/[sessionId]
 * Get breaks for a specific work session
 */
export const GET = createGetHandler<unknown>(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.sessionId)) {
      throw new Error("Invalid sessionId parameter");
    }
    const response = await apiGet<{ data: unknown }>(
      `/api/time-tracking/breaks/${params.sessionId}`,
      token,
    );
    return response.data;
  },
);
