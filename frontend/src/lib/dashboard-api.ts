import { fetchWithAuth } from "./fetch-with-auth";
import type { DashboardAnalyticsWithHome } from "./dashboard-helpers";

/**
 * Client-side fetcher for SWR — calls the Next.js BFF route
 */
export async function fetchDashboardAnalyticsClient(): Promise<DashboardAnalyticsWithHome> {
  const response = await fetchWithAuth("/api/dashboard/analytics");

  if (!response.ok) {
    throw new Error(`Dashboard fetch failed: ${response.status}`);
  }

  const json = (await response.json()) as { data: DashboardAnalyticsWithHome };
  return json.data;
}
