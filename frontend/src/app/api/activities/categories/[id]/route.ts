// app/api/activities/categories/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiPut, apiDelete } from "~/lib/api-helpers.server";
import {
  createPutHandler,
  createDeleteHandler,
} from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import {
  mapActivityCategoryResponse,
  toCategoryWriteBody,
} from "~/lib/activity-helpers";
import type {
  ActivityCategory,
  BackendActivityCategory,
  CategoryWriteRequest,
} from "~/lib/activity-helpers";

/**
 * Handler for PUT /api/activities/categories/[id]
 * Renames a category and updates its description/colour.
 */
export const PUT = createPutHandler<ActivityCategory, CategoryWriteRequest>(
  async (
    _request: NextRequest,
    body: CategoryWriteRequest,
    token: string,
    params,
  ) => {
    const response = await apiPut<{ data: BackendActivityCategory }>(
      `/api/activities/categories/${requirePathSegmentParam(params)}`,
      token,
      toCategoryWriteBody(body),
    );
    return mapActivityCategoryResponse(response.data);
  },
);

/**
 * Handler for DELETE /api/activities/categories/[id]
 * Archives the category. Nothing is deleted — existing Termine and
 * Aktivitäten keep their category and stay valid.
 */
export const DELETE = createDeleteHandler(
  async (_request: NextRequest, token: string, params) => {
    await apiDelete(
      `/api/activities/categories/${requirePathSegmentParam(params)}`,
      token,
    );
    return null;
  },
);
