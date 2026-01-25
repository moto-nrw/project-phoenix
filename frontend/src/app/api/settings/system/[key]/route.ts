// app/api/settings/system/[key]/route.ts
import type { NextRequest } from "next/server";
import { apiPut } from "~/lib/api-helpers";
import { createPutHandler, isStringParam } from "~/lib/route-wrapper";
import type { UpdateSettingRequest } from "~/lib/settings-helpers";

/**
 * Handler for PUT /api/settings/system/:key
 * Updates a system setting value (admin only)
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
    return await apiPut(`/api/settings/system/${params.key}`, token, body);
  },
);
