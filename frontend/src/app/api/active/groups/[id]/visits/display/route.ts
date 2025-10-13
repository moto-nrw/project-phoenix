// app/api/active/groups/[id]/visits/display/route.ts
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
 * Handler for GET /api/active/groups/[id]/visits/display
 * Returns visits with display data for a specific active group
 * Optimized bulk endpoint for SSE real-time updates
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string, params) => {
  if (!isStringParam(params.id)) {
    throw new Error('Invalid id parameter');
  }

  // Fetch group visits with display data from the API
  const response = await apiGet(`/api/active/groups/${params.id}/visits/display`, token);
  // Extract the data array from the backend response to avoid double-wrapping
  return response.data;
});
