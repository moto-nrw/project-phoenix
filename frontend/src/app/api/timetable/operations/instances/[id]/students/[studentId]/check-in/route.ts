import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler, isStringParam } from "~/lib/route-wrapper.server";

export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string, params) => {
    if (!isStringParam(params.id) || !isStringParam(params.studentId)) {
      throw new Error("Invalid id parameter");
    }
    const response = await apiPost<{ data: unknown }>(
      `/api/timetable/operations/instances/${params.id}/students/${params.studentId}/check-in`,
      token,
      body ?? {},
    );
    return response.data;
  },
);
