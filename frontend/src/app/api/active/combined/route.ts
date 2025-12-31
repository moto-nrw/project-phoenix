// app/api/active/combined/route.ts
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler, createProxyGetHandler } from "~/lib/route-wrapper";

/**
 * Type definition for combined group creation request
 */
interface CombinedGroupCreateRequest {
  name: string;
  description?: string;
  room_id?: string;
}

/**
 * Handler for GET /api/active/combined
 * Returns list of combined groups with optional filters
 */
export const GET = createProxyGetHandler("/api/active/combined");

/**
 * Handler for POST /api/active/combined
 * Creates a new combined group
 */
export const POST = createPostHandler<unknown, CombinedGroupCreateRequest>(
  async (
    _request: NextRequest,
    body: CombinedGroupCreateRequest,
    token: string,
  ) => {
    // Create the combined group via the API
    return await apiPost("/active/combined", token, body);
  },
);
