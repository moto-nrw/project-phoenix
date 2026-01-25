// app/api/settings/og/[ogId]/[key]/route.ts
import type { NextRequest } from "next/server";
import { apiDelete, apiPut } from "~/lib/api-helpers";
import {
  createDeleteHandler,
  createPutHandler,
  isStringParam,
} from "~/lib/route-wrapper";
import type { UpdateSettingRequest } from "~/lib/settings-helpers";

/**
 * Handler for PUT /api/settings/og/:ogId/:key
 * Updates an OG setting value
 */
export const PUT = createPutHandler<unknown, UpdateSettingRequest>(
  async (
    _request: NextRequest,
    body: UpdateSettingRequest,
    token: string,
    params,
  ) => {
    if (!isStringParam(params.ogId) || !isStringParam(params.key)) {
      throw new Error("Invalid ogId or key parameter");
    }
    return await apiPut(
      `/api/settings/og/${params.ogId}/${params.key}`,
      token,
      body,
    );
  },
);

/**
 * Handler for DELETE /api/settings/og/:ogId/:key
 * Resets an OG setting to inherit from parent scope
 */
export const DELETE = createDeleteHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.ogId) || !isStringParam(params.key)) {
      throw new Error("Invalid ogId or key parameter");
    }
    await apiDelete(`/api/settings/og/${params.ogId}/${params.key}`, token);
    return null;
  },
);
