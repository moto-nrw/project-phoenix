import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

/**
 * GET /api/time-tracking/history?from=YYYY-MM-DD&to=YYYY-MM-DD
 * Get work session history for date range
 */
export const GET = createGetHandler<unknown>(
  async (request: NextRequest, token: string) => {
    const searchParams = request.nextUrl.searchParams;
    const from = searchParams.get("from") ?? "";
    const to = searchParams.get("to") ?? "";

    const params = new URLSearchParams();
    if (from) params.append("from", from);
    if (to) params.append("to", to);

    const queryString = params.toString();
    const endpoint = queryString
      ? `/api/time-tracking/history?${queryString}`
      : "/api/time-tracking/history";

    const response = await apiGet(endpoint, token);
    return response;
  },
);
