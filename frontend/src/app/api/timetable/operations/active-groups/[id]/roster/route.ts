import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler, isStringParam } from "~/lib/route-wrapper";

export const GET = createGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) throw new Error("Invalid id parameter");
    const response = await apiGet<{ data: unknown }>(
      `/api/timetable/operations/active-groups/${params.id}/roster`,
      token,
    );
    return response.data;
  },
);
