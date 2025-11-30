// app/api/active/groups/[id]/visits/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

/**
 * Type guard to check if parameter exists and is a string
 */
function isStringParam(param: unknown): param is string {
  return typeof param === "string";
}

/**
 * Handler for GET /api/active/groups/[id]/visits
 * Returns visits for a specific active group
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }

    // Fetch group visits from the API
    return await apiGet(`/api/active/groups/${params.id}/visits`, token);
  },
);
