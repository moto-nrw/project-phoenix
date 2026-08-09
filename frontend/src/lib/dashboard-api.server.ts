import { apiGet } from "./api-helpers.server";
import type {
  DashboardAnalytics,
  DashboardAnalyticsResponse,
} from "./dashboard-helpers";
import { mapDashboardAnalyticsResponse } from "./dashboard-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DashboardAPI" });

/**
 * Fetches dashboard analytics data from the backend (server-side, used by BFF route)
 *
 * Every figure including studentsHome comes from the single analytics
 * endpoint. Deriving the home count here from /api/students would reintroduce
 * two problems: that endpoint is gated on `users:read` while this one needs
 * only `groups:read`, so the extra calls could 403 and take the whole
 * dashboard down; and a client-side subtraction double counts sick and
 * excused children, who never check in.
 *
 * @param token - JWT authentication token
 * @returns Promise<DashboardAnalytics>
 */
export async function fetchDashboardAnalytics(
  token: string,
): Promise<DashboardAnalytics> {
  try {
    const response = await apiGet<{ data: DashboardAnalyticsResponse }>(
      "/api/active/analytics/dashboard",
      token,
    );

    // The response is wrapped in a data property by common.Respond
    return mapDashboardAnalyticsResponse(response.data);
  } catch (error) {
    logger.error("failed to fetch dashboard analytics", {
      error: String(error),
    });
    // Re-throw the original error to preserve the 401 status
    throw error;
  }
}
