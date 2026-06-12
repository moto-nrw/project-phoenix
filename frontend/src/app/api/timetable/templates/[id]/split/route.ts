// app/api/timetable/templates/[id]/split/route.ts
//
// POST /api/timetable/templates/{id}/split
//   "Ab Datum ändern": ends the old template version at effective_date and
//   creates a new version with the submitted fields. Body shape mirrors
//   SplitTemplateBody in lib/timetable-types.ts.
//
// The Go backend wraps responses in { status, data, message }. Strip that
// envelope here so route-wrapper does not double-wrap the payload.
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler, isStringParam } from "~/lib/route-wrapper.server";

export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string, params) => {
    if (!isStringParam(params.id)) throw new Error("Invalid id parameter");
    const response = await apiPost<{ data: unknown }>(
      `/api/timetable/templates/${params.id}/split`,
      token,
      body ?? {},
    );
    return response.data;
  },
);
