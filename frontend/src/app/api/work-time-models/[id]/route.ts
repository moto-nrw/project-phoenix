import type { NextRequest } from "next/server";

import { apiDelete, apiGet, apiPut } from "~/lib/api-helpers.server";
import {
  createDeleteHandler,
  createGetHandler,
  createPutHandler,
} from "~/lib/route-wrapper.server";

type WorkTimeModel = Record<string, unknown>;
type UpdateBody = Record<string, unknown>;

export const GET = createGetHandler<WorkTimeModel>(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const response = await apiGet<{ data: WorkTimeModel }>(
      `/api/work-time-models/${id}`,
      token,
    );
    return response.data;
  },
);

export const PUT = createPutHandler<WorkTimeModel, UpdateBody>(
  async (
    _request: NextRequest,
    body: UpdateBody,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const response = await apiPut<{ data: WorkTimeModel }>(
      `/api/work-time-models/${id}`,
      token,
      body,
    );
    return response.data;
  },
);

export const DELETE = createDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    await apiDelete(`/api/work-time-models/${id}`, token);
    return null;
  },
);
