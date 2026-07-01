import {
  createParentGetHandler,
  parentApiGet,
} from "~/lib/parent/route-wrapper.server";

interface BackendMealPlanEntry {
  date: string;
  dish: string;
  note?: string | null;
}

/**
 * Proxy GET /api/parent/me/children/{studentId}/meal-plan → backend.
 * Returns the Monday-Friday meal plan for the child's school for the requested
 * week. The backend gates on operations.meal_plan_enabled for that tenant.
 */
export const GET = createParentGetHandler<BackendMealPlanEntry[]>(
  async (request, token, params) => {
    const studentId = String(params.studentId);
    const search = new URL(request.url).search;
    return parentApiGet<BackendMealPlanEntry[]>(
      `/parent/me/children/${encodeURIComponent(studentId)}/meal-plan${search}`,
      token,
    );
  },
);
