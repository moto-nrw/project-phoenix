// app/api/settings/og/[ogId]/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler, isStringParam } from "~/lib/route-wrapper";

/**
 * Handler for GET /api/settings/og/:ogId
 * Returns settings for a specific OG with resolved values
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.ogId)) {
      throw new Error("Invalid ogId parameter");
    }
    return await apiGet(`/api/settings/og/${params.ogId}`, token);
  },
);
