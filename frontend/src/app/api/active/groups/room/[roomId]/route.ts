// app/api/active/groups/room/[roomId]/route.ts
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
 * Handler for GET /api/active/groups/room/[roomId]
 * Returns active groups in a specific room
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string, params) => {
  if (!isStringParam(params.roomId)) {
    throw new Error('Invalid roomId parameter');
  }
  
  // Fetch active groups by room from the API
  return await apiGet(`/active/groups/room/${params.roomId}`, token);
});