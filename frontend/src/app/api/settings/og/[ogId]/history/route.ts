// app/api/settings/og/[ogId]/history/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler, isStringParam } from "~/lib/route-wrapper";

/**
 * Handler for GET /api/settings/og/:ogId/history
 * Returns setting change history for an OG
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string, params) => {
    if (!isStringParam(params.ogId)) {
      throw new Error("Invalid ogId parameter");
    }
    const queryString = request.nextUrl.search;
    return await apiGet(
      `/api/settings/og/${params.ogId}/history${queryString}`,
      token,
    );
  },
);
