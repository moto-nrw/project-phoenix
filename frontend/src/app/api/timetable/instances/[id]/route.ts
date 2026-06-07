import type { NextRequest } from "next/server";
import { apiDelete, apiPut } from "~/lib/api-helpers.server";
import {
  createDeleteHandler,
  createPutHandler,
  isStringParam,
} from "~/lib/route-wrapper.server";

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

export const DELETE = createDeleteHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) throw new Error("Invalid id parameter");
    await apiDelete(`/api/timetable/instances/${params.id}`, token);
    return null;
  },
);
