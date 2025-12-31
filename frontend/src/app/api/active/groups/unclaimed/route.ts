import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

/**
 * Handler for GET /api/active/groups/unclaimed
 * Returns all active groups that have no supervisors assigned
 * Used for deviceless rooms like Schulhof where teachers claim via frontend
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    // Fetch unclaimed groups from backend
    return await apiGet("/api/active/groups/unclaimed", token);
  },
);
