// app/api/activities/categories/[id]/restore/route.ts
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import type { BackendActivityCategory } from "~/lib/activity-helpers";

/**
 * Handler for POST /api/activities/categories/[id]/restore
 * Brings an archived category back into the pickers. The backend rejects the
 * restore with a 409 when an active category has taken the name meanwhile.
 */
export const POST = createPostHandler<BackendActivityCategory, unknown>(
  async (_request: NextRequest, _body: unknown, token: string, params) => {
    const response = await apiPost<{ data: BackendActivityCategory }>(
      `/api/activities/categories/${requirePathSegmentParam(params)}/restore`,
      token,
    );
    return response.data;
  },
);
