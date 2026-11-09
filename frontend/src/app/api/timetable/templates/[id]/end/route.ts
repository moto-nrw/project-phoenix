// app/api/timetable/templates/[id]/end/route.ts
//
// POST /api/timetable/templates/{id}/end
//   Ends a recurring template at effective_date without creating a successor.
import {
  encodePathSegment,
  requireStringParam,
} from "~/lib/route-wrapper-utils.server";
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string, params) => {
    const id = requireStringParam(params, "id");
    const response = await apiPost<{ data: unknown }>(
      `/api/timetable/templates/${encodePathSegment(id)}/end`,
      token,
      body ?? {},
    );
    return response.data;
  },
);
