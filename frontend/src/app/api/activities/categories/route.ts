// app/api/activities/categories/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";
import { buildQueryString } from "~/lib/route-wrapper-utils.server";
import type {
  ActivityCategory,
  BackendActivityCategory,
  CategoryWriteRequest,
} from "~/lib/activity-helpers";
import {
  mapActivityCategoryResponse,
  toCategoryWriteBody,
} from "~/lib/activity-helpers";

/**
 * Handler for GET /api/activities/categories
 * Returns the tenant's activity categories.
 *
 * The query string is forwarded verbatim so callers can opt into the two
 * backend filters: `include_system=true` (auto-provisioned Schulhof/WC) and
 * `include_archived=true` (retired categories, needed by the Stammdaten
 * screen). Without them the response is exactly the pickable set — what the
 * Termin and Aktivitäten forms want.
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const response = await apiGet<{
      status: string;
      data: BackendActivityCategory[];
    }>(`/api/activities/categories${buildQueryString(request)}`, token);

    // Handle response structure
    if (response?.status === "success" && Array.isArray(response.data)) {
      return response.data.map(mapActivityCategoryResponse);
    }

    // If we get here, we have a response but it's not in the expected format
    throw new Error("Unexpected response structure from categories API");
  },
);

/**
 * Handler for POST /api/activities/categories
 * Creates a school-owned activity category. Requires
 * `activities:manage_categories` on the backend.
 */
export const POST = createPostHandler<ActivityCategory, CategoryWriteRequest>(
  async (_request: NextRequest, body: CategoryWriteRequest, token: string) => {
    const response = await apiPost<{ data: BackendActivityCategory }>(
      "/api/activities/categories",
      token,
      toCategoryWriteBody(body),
    );
    // Mapped like GET so the whole /api/activities/categories surface speaks
    // the frontend ActivityCategory shape, not raw snake_case.
    return mapActivityCategoryResponse(response.data);
  },
);
