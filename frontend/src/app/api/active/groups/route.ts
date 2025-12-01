// app/api/active/groups/route.ts
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler, createProxyGetHandler } from "~/lib/route-wrapper";

/**
 * Type definition for group creation request
 */
interface GroupCreateRequest {
  name: string;
  description?: string;
  room_id?: string;
}

/**
 * Handler for GET /api/active/groups
 * Returns a list of active groups, optionally filtered by query parameters
 */
export const GET = createProxyGetHandler("/api/active/groups");

/**
 * Handler for POST /api/active/groups
 * Creates a new active group
 */
export const POST = createPostHandler<unknown, GroupCreateRequest>(
  async (_request: NextRequest, body: GroupCreateRequest, token: string) => {
    // Create the active group via the API
    return await apiPost("/api/active/groups", token, body);
  },
);
