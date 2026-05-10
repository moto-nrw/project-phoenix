import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler, isStringParam } from "~/lib/route-wrapper";

export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string, params) => {
    if (!isStringParam(params.id)) throw new Error("Invalid id parameter");
    const response = await apiPost<{ data: unknown }>(
      `/api/timetable/operations/instances/${params.id}/complete`,
      token,
      body ?? {},
    );
    return response.data;
  },
);
