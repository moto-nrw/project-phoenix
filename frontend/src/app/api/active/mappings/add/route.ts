// app/api/active/mappings/add/route.ts
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

/**
 * Type definition for adding a group to a combined group
 */
interface AddMappingRequest {
  combined_id: string;
  group_id: string;
}

/**
 * Handler for POST /api/active/mappings/add
 * Adds a group to a combined group
 */
export const POST = createPostHandler<unknown, AddMappingRequest>(
  async (_request: NextRequest, body: AddMappingRequest, token: string) => {
    // Add group to combined group via the API
    return await apiPost("/active/mappings/add", token, body);
  },
);
