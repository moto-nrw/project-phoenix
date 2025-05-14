// app/api/active/combined/[id]/end/route.ts
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

/**
 * Handler for POST /api/active/combined/[id]/end
 * Ends an active combined group
 */
export const POST = createPostHandler(async (_request: NextRequest, _body: Record<string, never>, token: string, params) => {
  const id = params.id as string;
  
  // End the combined group via the API
  return await apiPost(`/active/combined/${id}/end`, token);
});