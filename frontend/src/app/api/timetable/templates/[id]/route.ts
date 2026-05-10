import type { NextRequest } from "next/server";
import { apiDelete, apiGet, apiPut } from "~/lib/api-helpers";
import {
  createDeleteHandler,
  createGetHandler,
  createPutHandler,
  isStringParam,
} from "~/lib/route-wrapper";

export const GET = createGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) throw new Error("Invalid id parameter");
    const response = await apiGet<{ data: unknown }>(
      `/api/timetable/templates/${params.id}`,
      token,
    );
    return response.data;
  },
);

export const PUT = createPutHandler(
  async (_request: NextRequest, body: unknown, token: string, params) => {
    if (!isStringParam(params.id)) throw new Error("Invalid id parameter");
    const response = await apiPut<{ data: unknown }>(
      `/api/timetable/templates/${params.id}`,
      token,
      body ?? {},
    );
    return response.data;
  },
);

export const DELETE = createDeleteHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) throw new Error("Invalid id parameter");
    await apiDelete(`/api/timetable/templates/${params.id}`, token);
    return null;
  },
);
