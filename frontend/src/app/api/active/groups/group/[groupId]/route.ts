// app/api/active/groups/group/[groupId]/route.ts
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
 * Handler for GET /api/active/groups/group/[groupId]
 * Returns active groups for a specific education group
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.groupId)) {
      throw new Error("Invalid groupId parameter");
    }

    // Fetch active groups by education group from the API
    return await apiGet(`/active/groups/group/${params.groupId}`, token);
  },
);
