// app/api/groups/[id]/students/room-status/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

/**
 * Handler for GET /api/groups/[id]/students/room-status
 * Gets room status for all students in an educational group
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    if (!id) {
      throw new Error("Group ID is required");
    }

    // Fetch room status from backend API
    const response = await apiGet<unknown>(
      `/api/groups/${id}/students/room-status`,
      token,
    );

    // Type guard to check response structure
    if (!response || typeof response !== "object" || !("data" in response)) {
      throw new Error("Invalid response format");
    }

    const typedResponse = response as { data: unknown };

    // Return the room status data
    return typedResponse.data;
  },
);
