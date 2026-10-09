import { apiErrorFromResponse, unavailableApiError } from "./api-error";
import { fetchWithAuth } from "./fetch-with-auth";
import type { DashboardAnalytics } from "./dashboard-helpers";

/**
 * Client-side fetcher for SWR — calls the Next.js BFF route
 */
export async function fetchDashboardAnalyticsClient(): Promise<DashboardAnalytics> {
  let response: Response;
  try {
    response = await fetchWithAuth("/api/dashboard/analytics");
  } catch (error) {
    throw unavailableApiError(error);
  }

  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      `Dashboard fetch failed: ${response.status}`,
    );
  }

  const json = (await response.json()) as { data: DashboardAnalytics };
  return json.data;
}
