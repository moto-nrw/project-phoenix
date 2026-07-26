// app/api/timetable/templates/[id]/end/route.ts
//
// POST /api/timetable/templates/{id}/end
//   Ends a recurring template at effective_date without creating a successor.
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler, isStringParam } from "~/lib/route-wrapper.server";

export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string, params) => {
    if (!isStringParam(params.id)) throw new Error("Invalid id parameter");
    const response = await apiPost<{ data: unknown }>(
      `/api/timetable/templates/${params.id}/end`,
      token,
      body ?? {},
    );
    return response.data;
  },
);
