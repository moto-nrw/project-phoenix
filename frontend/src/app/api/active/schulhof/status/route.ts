import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

/**
 * Handler for GET /api/active/schulhof/status
 * Returns the current Schulhof status including:
 * - Whether infrastructure exists
 * - Room and activity group IDs
 * - Current supervisors and student count
 * - Whether the current user is supervising
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    _params: Record<string, unknown>,
  ) => {
    return await apiGet("/api/active/schulhof/status", token);
  },
);
