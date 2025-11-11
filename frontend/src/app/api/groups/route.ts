import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";
import type { BackendGroup } from "~/lib/group-helpers";

/**
 * Type for paginated response from backend
 */
interface PaginatedGroupResponse {
  status: string;
  data: BackendGroup[];
  pagination: {
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  };
  message?: string;
}

/**
 * Type definition for group creation request
 */
interface GroupCreateRequest {
  name: string;
  description?: string;
  room_id?: number;
  representative_id?: number;
  teacher_ids?: number[];
}

/**
 * Handler for GET /api/groups
 * Returns a list of groups, optionally filtered by query parameters
 */
export const GET = createGetHandler(async (request: NextRequest, token: string) => {
  // Build URL with any query parameters
  const queryParams = new URLSearchParams();
  request.nextUrl.searchParams.forEach((value, key) => {
    queryParams.append(key, value);
  });
  
  const endpoint = `/api/groups${queryParams.toString() ? '?' + queryParams.toString() : ''}`;
  
  // Fetch groups from the API - backend returns paginated response
  const paginatedResponse = await apiGet<PaginatedGroupResponse>(endpoint, token);

  // For now, return just the data array to match frontend expectations
  // TODO: Update frontend to handle pagination metadata
  return paginatedResponse.data || [];
});

/**
 * Handler for POST /api/groups
 * Creates a new group
 */
export const POST = createPostHandler(async (req: NextRequest, body: GroupCreateRequest, token: string) => {
  const endpoint = `/api/groups`;
  
  // Create group via the API
  return await apiPost<BackendGroup>(endpoint, token, body);
});