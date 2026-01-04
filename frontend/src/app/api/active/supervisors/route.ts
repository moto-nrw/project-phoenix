// app/api/active/supervisors/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost, extractParams } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";

/**
 * Type definition for supervisor creation request
 */
interface SupervisorCreateRequest {
  staff_id: string;
  active_group_id: string;
  // Add any other required fields
}

/**
 * Handler for GET /api/active/supervisors
 * Returns list of active supervisors, with optional query filters
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string, params) => {
    // Extract query params
    const queryParams = extractParams(request, params);

    // Construct query string
    let endpoint = "/active/supervisors";
    const queryString = new URLSearchParams(queryParams).toString();
    if (queryString) {
      endpoint += `?${queryString}`;
    }

    // Fetch supervisors from the API
    return await apiGet(endpoint, token);
  },
);

/**
 * Handler for POST /api/active/supervisors
 * Creates a new supervisor
 */
export const POST = createPostHandler<unknown, SupervisorCreateRequest>(
  async (
    _request: NextRequest,
    body: SupervisorCreateRequest,
    token: string,
    _params,
  ) => {
    // Create a new supervisor via the API
    return await apiPost("/active/supervisors", token, body);
  },
);
