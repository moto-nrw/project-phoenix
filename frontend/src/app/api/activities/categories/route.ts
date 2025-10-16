// app/api/activities/categories/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";
import type { BackendActivityCategory } from "~/lib/activity-helpers";
import { mapActivityCategoryResponse } from "~/lib/activity-helpers";

/**
 * Handler for GET /api/activities/categories
 * Returns a list of activity categories
 */
export const GET = createGetHandler(async (request: NextRequest, token: string) => {
  const response = await apiGet<{ status: string; data: BackendActivityCategory[] }>('/api/activities/categories', token);
  
  // Handle response structure
  if (response?.status === "success" && Array.isArray(response.data)) {
    return response.data.map(mapActivityCategoryResponse);
  }
  
  // If we get here, we have a response but it's not in the expected format
  throw new Error('Unexpected response structure from categories API');
});