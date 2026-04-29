import type { NextRequest } from "next/server";
import { apiPatch } from "~/lib/api-helpers";
import { createPatchHandler, isStringParam } from "~/lib/route-wrapper";

export const PATCH = createPatchHandler(
  async (_request: NextRequest, body: unknown, token: string, params) => {
    if (!isStringParam(params.id) || !isStringParam(params.studentId)) {
      throw new Error("Invalid id parameter");
    }
    const response = await apiPatch<{ data: unknown }>(
      `/api/timetable/instances/${params.id}/students/${params.studentId}`,
      token,
      body ?? {},
    );
    return response.data;
  },
);
