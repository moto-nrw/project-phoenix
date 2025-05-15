// app/api/active/visits/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";
import { extractParams } from "~/lib/api-helpers";

/**
 * Type definition for visit creation request
 */
interface VisitCreateRequest {
  student_id: string;
  active_group_id: string;
  start_time?: string;
  // Add any other required fields
}

/**
 * Handler for GET /api/active/visits
 * Returns list of active visits, with optional query filters
 */
export const GET = createGetHandler(async (request: NextRequest, token: string, params) => {
  // Extract query params
  const queryParams = extractParams(request, params);
  
  // Construct query string
  let endpoint = '/active/visits';
  const queryString = new URLSearchParams(queryParams).toString();
  if (queryString) {
    endpoint += `?${queryString}`;
  }
  
  // Fetch visits from the API
  return await apiGet(endpoint, token);
});

/**
 * Handler for POST /api/active/visits
 * Creates a new visit
 */
export const POST = createPostHandler<unknown, VisitCreateRequest>(
  async (_request: NextRequest, body: VisitCreateRequest, token: string, _params) => {
    // Create a new visit via the API
    return await apiPost('/active/visits', token, body);
  }
);