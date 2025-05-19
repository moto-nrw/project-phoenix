// app/api/active/analytics/room/[roomId]/utilization/route.ts
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
 * Handler for GET /api/active/analytics/room/[roomId]/utilization
 * Returns utilization data for a specific room
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string, params) => {
  if (!isStringParam(params.roomId)) {
    throw new Error('Invalid roomId parameter');
  }
  
  // Fetch room utilization data from the API
  return await apiGet(`/active/analytics/room/${params.roomId}/utilization`, token);
});