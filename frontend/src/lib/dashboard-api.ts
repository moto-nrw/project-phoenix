import { apiGet } from "./api-helpers";
import { fetchWithAuth } from "./fetch-with-auth";
import type {
  DashboardAnalytics,
  DashboardAnalyticsResponse,
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

/**
 * Client-side fetcher for SWR — calls the Next.js BFF route
 */
export async function fetchDashboardAnalyticsClient(): Promise<DashboardAnalytics> {
  const response = await fetchWithAuth("/api/dashboard/analytics");

  if (!response.ok) {
    throw new Error(`Dashboard fetch failed: ${response.status}`);
  }

  const json = (await response.json()) as { data: DashboardAnalytics };
  return json.data;
}
