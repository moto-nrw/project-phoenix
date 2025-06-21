import { apiGet } from "./api-helpers";
import type { DashboardAnalytics, DashboardAnalyticsResponse } from "./dashboard-helpers";
import { mapDashboardAnalyticsResponse } from "./dashboard-helpers";

/**
 * Fetches dashboard analytics data from the backend
 * @param token - JWT authentication token
 * @returns Promise<DashboardAnalytics>
 */
export async function fetchDashboardAnalytics(
  token: string
): Promise<DashboardAnalytics> {
  try {
    const response = await apiGet<{ data: DashboardAnalyticsResponse }>("/api/active/analytics/dashboard", token);
    
    // The response is wrapped in a data property by common.Respond
    return mapDashboardAnalyticsResponse(response.data);
  } catch (error) {
    console.error("Error fetching dashboard analytics:", error);
    // Re-throw the original error to preserve the 401 status
    throw error;
  }
}