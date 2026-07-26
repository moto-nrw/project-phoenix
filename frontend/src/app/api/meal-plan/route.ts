import type { NextRequest } from "next/server";

import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import type { BackendMealPlanEntry } from "~/lib/meal-plan-api";

/**
 * GET /api/meal-plan?week_start=YYYY-MM-DD
 * Returns the Monday-Friday meal plan entries for the week containing
 * week_start. Empty days are simply absent from the array.
 */
export const GET = createGetHandler<BackendMealPlanEntry[]>(
  async (request: NextRequest, token: string) => {
    const weekStart = request.nextUrl.searchParams.get("week_start") ?? "";
    const response = await apiGet<{ data: BackendMealPlanEntry[] }>(
      `/api/meal-plan?week_start=${encodeURIComponent(weekStart)}`,
      token,
    );
    return response.data ?? [];
  },
);
