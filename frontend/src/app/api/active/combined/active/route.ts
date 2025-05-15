// app/api/active/combined/active/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

/**
 * Handler for GET /api/active/combined/active
 * Returns active combined groups
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string) => {
  // Fetch active combined groups from the API
  return await apiGet("/active/combined/active", token);
});