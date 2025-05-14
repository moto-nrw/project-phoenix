// app/api/active/analytics/room/[roomId]/utilization/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

/**
 * Handler for GET /api/active/analytics/room/[roomId]/utilization
 * Returns utilization data for a specific room
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string, params) => {
  const roomId = params.roomId as string;
  
  // Fetch room utilization data from the API
  return await apiGet(`/active/analytics/room/${roomId}/utilization`, token);
});