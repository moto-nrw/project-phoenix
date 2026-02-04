import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler, isStringParam } from "~/lib/route-wrapper";

/**
 * GET /api/time-tracking/{id}/edits
 * Fetch audit trail for a work session
 */
export const GET = createGetHandler<unknown>(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid session ID");
    }

    const response = await apiGet<{ data: unknown }>(
      `/api/time-tracking/${params.id}/edits`,
      token,
    );
    return response.data;
  },
);
