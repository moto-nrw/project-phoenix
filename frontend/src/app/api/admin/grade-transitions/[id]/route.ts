// app/api/admin/grade-transitions/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPutHandler,
  createDeleteHandler,
} from "~/lib/route-wrapper.server";
import type { BackendGradeTransition } from "~/lib/grade-transition-api";

interface DetailResponse {
  data: BackendGradeTransition;
}

function requireId(params: Record<string, unknown>): string {
  const id = params.id;
  if (typeof id !== "string" || !/^\d+$/.test(id)) {
    throw new Error("Invalid id parameter");
  }
  return id;
}

export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token,
    params,
  ): Promise<BackendGradeTransition> => {
    const id = requireId(params);
    const response = await apiGet<DetailResponse>(
      `/api/admin/grade-transitions/${id}`,
      token,
    );
    return response.data;
  },
);

export const PUT = createPutHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token,
    params,
  ): Promise<BackendGradeTransition> => {
    const id = requireId(params);
    const response = await apiPut<DetailResponse>(
      `/api/admin/grade-transitions/${id}`,
      token,
      body,
    );
    return response.data;
  },
);

export const DELETE = createDeleteHandler(
  async (_request: NextRequest, token, params): Promise<null> => {
    const id = requireId(params);
    await apiDelete(`/api/admin/grade-transitions/${id}`, token);
    return null;
  },
);
