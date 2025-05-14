// app/api/active/combined/[id]/groups/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

/**
 * Handler for GET /api/active/combined/[id]/groups
 * Returns the groups in a combined group
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string, params) => {
  const id = params.id as string;
  
  // Fetch groups in the combined group via the API
  return await apiGet(`/active/combined/${id}/groups`, token);
});