// app/api/active/groups/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers";
import {
  createGetHandler,
  createPutHandler,
  createDeleteHandler,
} from "~/lib/route-wrapper";

/**
 * Type definition for group update request
 */
interface GroupUpdateRequest {
  name?: string;
  description?: string;
  room_id?: string;
  // Add other properties as needed
}

/**
 * Type guard to check if parameter exists and is a string
 */
function isStringParam(param: unknown): param is string {
  return typeof param === "string";
}

/**
 * Handler for GET /api/active/groups/[id]
 * Returns details of a specific active group
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }

    // Fetch active group details from the API
    return await apiGet(`/active/groups/${params.id}`, token);
  },
);

/**
 * Handler for PUT /api/active/groups/[id]
 * Updates an active group
 */
export const PUT = createPutHandler<unknown, GroupUpdateRequest>(
  async (
    _request: NextRequest,
    body: GroupUpdateRequest,
    token: string,
    params,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }

    // Update the active group via the API
    return await apiPut(`/active/groups/${params.id}`, token, body);
  },
);

/**
 * Handler for DELETE /api/active/groups/[id]
 * Deletes an active group
 */
export const DELETE = createDeleteHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }

    // Delete the active group via the API
    await apiDelete(`/active/groups/${params.id}`, token);

    // Return 204 No Content response
    return null;
  },
);
