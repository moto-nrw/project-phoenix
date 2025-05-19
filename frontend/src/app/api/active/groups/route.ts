// app/api/active/groups/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";

/**
 * Type definition for group creation request
 */
interface GroupCreateRequest {
  name: string;
  description?: string;
  room_id?: string;
  // Add other properties as needed
}

/**
 * Handler for GET /api/active/groups
 * Returns a list of active groups, optionally filtered by query parameters
 */
export const GET = createGetHandler(async (request: NextRequest, token: string) => {
  // Build URL with any query parameters
  const queryParams = new URLSearchParams();
  request.nextUrl.searchParams.forEach((value, key) => {
    queryParams.append(key, value);
  });
  
  const endpoint = `/active/groups${queryParams.toString() ? '?' + queryParams.toString() : ''}`;
  
  // Fetch active groups from the API
  return await apiGet(endpoint, token);
});

/**
 * Handler for POST /api/active/groups
 * Creates a new active group
 */
export const POST = createPostHandler<unknown, GroupCreateRequest>(
  async (_request: NextRequest, body: GroupCreateRequest, token: string) => {
    // Create the active group via the API
    return await apiPost("/active/groups", token, body);
  }
);