import type { NextRequest } from "next/server";

import { apiDelete, apiPut } from "~/lib/api-helpers.server";
import {
  createDeleteHandler,
  createPutHandler,
} from "~/lib/route-wrapper.server";
import type { BackendMealPlanEntry } from "~/lib/meal-plan-api";

type SetDayBody = { dishes: { dish: string; note: string | null }[] };

/**
 * PUT /api/meal-plan/{date} — replace all dishes for one day.
 */
export const PUT = createPutHandler<BackendMealPlanEntry, SetDayBody>(
  async (
    _request: NextRequest,
    body: SetDayBody,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const date = params.date as string;
    const response = await apiPut<{ data: BackendMealPlanEntry }>(
      `/api/meal-plan/${encodeURIComponent(date)}`,
      token,
      body,
    );
    return response.data;
  },
);

/**
 * DELETE /api/meal-plan/{date} — remove the meal for one day.
 */
export const DELETE = createDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const date = params.date as string;
    await apiDelete(`/api/meal-plan/${encodeURIComponent(date)}`, token);
    return null;
  },
);
