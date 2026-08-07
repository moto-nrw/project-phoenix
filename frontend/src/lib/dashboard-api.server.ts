import { apiGet } from "./api-helpers.server";
import type {
  DashboardAnalyticsResponse,
  DashboardAnalyticsWithHome,
} from "./dashboard-helpers";
import { mapDashboardAnalyticsResponse } from "./dashboard-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DashboardAPI" });

/**
 * Reads pagination.total_records out of a /api/students response.
 *
 * That endpoint ships two envelopes (see parseStudentsPaginatedResponse in
 * lib/api.ts): the wrapped `{ data: { data, pagination } }` form and the
 * direct `{ data, pagination }` form — and `pagination` is optional in both.
 * An unguarded deref therefore throws a TypeError that the caller would
 * re-throw as if it were the 401 it means to preserve.
 */
function readTotalRecords(response: unknown): number | null {
  if (!response || typeof response !== "object") return null;

  const direct = (response as { pagination?: { total_records?: unknown } })
    .pagination;
  const wrapped = (
    response as { data?: { pagination?: { total_records?: unknown } } }
  ).data?.pagination;

  const total = direct?.total_records ?? wrapped?.total_records;
  return typeof total === "number" && Number.isFinite(total) ? total : null;
}

/**
 * Counts students via /api/students without letting a failure take the whole
 * dashboard down. The endpoint is gated on `users:read` while the analytics
 * endpoint is gated on `groups:read`, so a role holding only the latter gets a
 * 403 here — that must degrade the "Zuhause" tile, not error-state every
 * number on the page.
 */
async function fetchStudentCount(
  query: string,
  token: string,
): Promise<number | null> {
  try {
    return readTotalRecords(await apiGet<unknown>(query, token));
  } catch (error) {
    logger.warn("failed to fetch student count for dashboard", {
      query,
      error: String(error),
    });
    return null;
  }
}

/**
 * Fetches dashboard analytics data from the backend (server-side, used by BFF route)
 * @param token - JWT authentication token
 * @returns Promise<DashboardAnalytics>
 */
export async function fetchDashboardAnalytics(
  token: string,
): Promise<DashboardAnalyticsWithHome> {
  try {
    const [response, totalStudents, presentStudents] = await Promise.all([
      apiGet<{ data: DashboardAnalyticsResponse }>(
        "/api/active/analytics/dashboard",
        token,
      ),
      fetchStudentCount("/api/students?page=1&page_size=1", token),
      fetchStudentCount(
        "/api/students?location_state=present&page=1&page_size=1",
        token,
      ),
    ]);

    // Only report a home count when both halves of the subtraction are known —
    // a missing count would otherwise silently render as "everyone is home".
    const studentsHome =
      totalStudents !== null && presentStudents !== null
        ? Math.max(0, totalStudents - presentStudents)
        : null;

    // The response is wrapped in a data property by common.Respond
    return {
      ...mapDashboardAnalyticsResponse(response.data),
      studentsHome,
    };
  } catch (error) {
    logger.error("failed to fetch dashboard analytics", {
      error: String(error),
    });
    // Re-throw the original error to preserve the 401 status
    throw error;
  }
}
