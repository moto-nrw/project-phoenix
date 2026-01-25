// app/api/settings/user/me/[key]/route.ts
import type { NextRequest } from "next/server";
import { apiPut } from "~/lib/api-helpers";
import { createPutHandler, isStringParam } from "~/lib/route-wrapper";
import type { UpdateSettingRequest } from "~/lib/settings-helpers";

/**
 * Handler for PUT /api/settings/user/me/:key
 * Updates a user setting value
 */
export const PUT = createPutHandler<unknown, UpdateSettingRequest>(
  async (
    _request: NextRequest,
    body: UpdateSettingRequest,
    token: string,
    params,
  ) => {
    if (!isStringParam(params.key)) {
      throw new Error("Invalid key parameter");
    }
    return await apiPut(`/api/settings/user/me/${params.key}`, token, body);
  },
);
