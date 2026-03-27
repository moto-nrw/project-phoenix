import type { NextRequest } from "next/server";
import { apiPut, apiDelete } from "~/lib/api-helpers";
import { createPutHandler, createDeleteHandler } from "~/lib/route-wrapper";

export const PUT = createPutHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const key = params.key as string;
    return await apiPut(`/api/settings/values/${key}`, token, body);
  },
);

export const DELETE = createDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const key = params.key as string;
    return await apiDelete(`/api/settings/values/${key}`, token);
  },
);
