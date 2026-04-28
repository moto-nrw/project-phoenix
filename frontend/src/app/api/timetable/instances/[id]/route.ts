import type { NextRequest } from "next/server";
import { apiPut } from "~/lib/api-helpers";
import { createPutHandler, isStringParam } from "~/lib/route-wrapper";

export const PUT = createPutHandler(
  async (_request: NextRequest, body: unknown, token: string, params) => {
    if (!isStringParam(params.id)) throw new Error("Invalid id parameter");
    const response = await apiPut<{ data: unknown }>(
      `/api/timetable/instances/${params.id}`,
      token,
      body ?? {},
    );
    return response.data;
  },
);
