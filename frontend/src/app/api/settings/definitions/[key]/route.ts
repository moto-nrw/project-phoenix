// app/api/settings/definitions/[key]/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler, isStringParam } from "~/lib/route-wrapper";

/**
 * Handler for GET /api/settings/definitions/:key
 * Returns a single setting definition by key
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.key)) {
      throw new Error("Invalid key parameter");
    }
    return await apiGet(`/api/settings/definitions/${params.key}`, token);
  },
);
