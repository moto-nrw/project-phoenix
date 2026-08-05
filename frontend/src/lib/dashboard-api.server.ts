import { apiGet } from "./api-helpers.server";
import type {
  DashboardAnalyticsResponse,
  DashboardAnalyticsWithHome,
} from "./dashboard-helpers";
import { mapDashboardAnalyticsResponse } from "./dashboard-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DashboardAPI" });

/**
 * Fetches dashboard analytics data from the backend (server-side, used by BFF route)
 * @param token - JWT authentication token
 * @returns Promise<DashboardAnalytics>
 */
export async function fetchDashboardAnalytics(
  token: string,
): Promise<DashboardAnalyticsWithHome> {
  try {
    const [response, studentResponse, presentStudentResponse] =
      await Promise.all([
        apiGet<{ data: DashboardAnalyticsResponse }>(
          "/api/active/analytics/dashboard",
          token,
        ),
        apiGet<{ pagination: { total_records: number } }>(
          "/api/students?page=1&page_size=1",
          token,
        ),
        apiGet<{ pagination: { total_records: number } }>(
          "/api/students?location_state=present&page=1&page_size=1",
          token,
        ),
      ]);

    // The response is wrapped in a data property by common.Respond
    return {
      ...mapDashboardAnalyticsResponse(response.data),
      studentsHome:
        studentResponse.pagination.total_records -
        presentStudentResponse.pagination.total_records,
    };
  } catch (error) {
    logger.error("failed to fetch dashboard analytics", {
      error: String(error),
    });
    // Re-throw the original error to preserve the 401 status
    throw error;
  }
}
