// app/api/active/mappings/group/[groupId]/route.ts
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
 * Handler for GET /api/active/mappings/group/[groupId]
 * Returns mappings for a specific group
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.groupId)) {
      throw new Error("Invalid groupId parameter");
    }

    // Fetch group mappings from the API
    return await apiGet(`/active/mappings/group/${params.groupId}`, token);
  },
);
