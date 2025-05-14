// app/api/active/mappings/combined/[combinedId]/route.ts
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
 * Handler for GET /api/active/mappings/combined/[combinedId]
 * Returns mappings for a specific combined group
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string, params) => {
  if (!isStringParam(params.combinedId)) {
    throw new Error('Invalid combinedId parameter');
  }
  
  // Fetch combined group mappings from the API
  return await apiGet(`/active/mappings/combined/${params.combinedId}`, token);
});