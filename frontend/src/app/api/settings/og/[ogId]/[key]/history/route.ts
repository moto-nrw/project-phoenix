// app/api/settings/og/[ogId]/[key]/history/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler, isStringParam } from "~/lib/route-wrapper";

/**
 * Handler for GET /api/settings/og/:ogId/:key/history
 * Returns setting change history for a specific key in an OG
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string, params) => {
    if (!isStringParam(params.ogId) || !isStringParam(params.key)) {
      throw new Error("Invalid ogId or key parameter");
    }
    const queryString = request.nextUrl.search;
    return await apiGet(
      `/api/settings/og/${params.ogId}/${params.key}/history${queryString}`,
      token,
    );
  },
);
