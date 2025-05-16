// app/api/active/analytics/counts/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

/**
 * Handler for GET /api/active/analytics/counts
 * Returns counts of active groups, visits, and supervisors
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string, _params: Record<string, unknown>) => {
  // Fetch counts from the API
  return await apiGet("/active/analytics/counts", token);
});