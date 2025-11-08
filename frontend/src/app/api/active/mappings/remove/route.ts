// app/api/active/mappings/remove/route.ts
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

/**
 * Type definition for remove mapping request
 */
interface RemoveMappingRequest {
  combined_id: string;
  group_id: string;
}

/**
 * Handler for POST /api/active/mappings/remove
 * Removes a group from a combined group
 */
export const POST = createPostHandler<unknown, RemoveMappingRequest>(
  async (
    _request: NextRequest,
    body: RemoveMappingRequest,
    token: string,
    _params,
  ) => {
    // Remove group mapping via the API
    return await apiPost("/active/mappings/remove", token, body);
  },
);
