// app/api/active/combined/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";

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
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    // Construct a URL with all query parameters
    const queryParams = new URLSearchParams();
    request.nextUrl.searchParams.forEach((value, key) => {
      queryParams.append(key, value);
    });

    const endpoint = `/api/active/combined${queryParams.toString() ? "?" + queryParams.toString() : ""}`;

    // Fetch combined groups from the API
    return await apiGet(endpoint, token);
  },
);

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
