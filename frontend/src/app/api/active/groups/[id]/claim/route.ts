import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

/**
 * Handler for POST /api/active/groups/{id}/claim
 * Allows authenticated staff to claim supervision of an active group
 * Used for deviceless rooms like Schulhof
 */
export const POST = createPostHandler(
  async (
    _request: NextRequest,
    _body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const { id } = params;

    if (typeof id !== "string") {
      throw new Error("Invalid group ID");
    }

    // Claim the group with default supervisor role
    const requestBody = { role: "supervisor" };

    return await apiPost(`/api/active/groups/${id}/claim`, token, requestBody);
  },
);
