import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface BackendMealPlanEntry {
  date: string;
  dish: string;
  note?: string | null;
}

/**
 * Proxy GET /api/parent/me/children/{studentId}/meal-plan → backend.
 * Returns the Monday-Friday meal plan for the child's school for the requested
 * week (query string is forwarded). The backend gates on
 * operations.meal_plan_enabled for that tenant.
 */
export const GET = proxyGet<BackendMealPlanEntry[]>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/meal-plan`,
);
