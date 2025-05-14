// app/api/active/visits/group/[groupId]/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

/**
 * Type guard to check if parameter exists and is a string
 */
function isStringParam(param: unknown): param is string {
  return typeof param === 'string';
}

/**
 * Handler for GET /api/active/visits/group/[groupId]
 * Returns visits for a specific group
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string, params) => {
  if (!isStringParam(params.groupId)) {
    throw new Error('Invalid groupId parameter');
  }
  
  // Fetch visits for the group from the API
  return await apiGet(`/active/visits/group/${params.groupId}`, token);
});