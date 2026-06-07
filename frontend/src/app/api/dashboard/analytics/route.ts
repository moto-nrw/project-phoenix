import type { NextRequest } from "next/server";
import { fetchDashboardAnalytics } from "~/lib/dashboard-api.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Handler for GET /api/dashboard/analytics
 * Returns aggregated dashboard analytics data
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    _params: Record<string, unknown>,
  ) => {
    // Fetch dashboard analytics from the backend
    return await fetchDashboardAnalytics(token);
  },
);
