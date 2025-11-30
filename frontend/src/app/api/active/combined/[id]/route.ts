// app/api/active/combined/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiDelete, apiGet, apiPut } from "~/lib/api-helpers";
import {
  createDeleteHandler,
  createGetHandler,
  createPutHandler,
} from "~/lib/route-wrapper";

/**
 * Type definition for combined group update request
 */
interface CombinedGroupUpdateRequest {
  name?: string;
  description?: string;
  room_id?: string;
}

/**
 * Type guard to check if parameter exists and is a string
 */
function isStringParam(param: unknown): param is string {
  return typeof param === "string";
}

/**
 * Handler for GET /api/active/combined/[id]
 * Returns details of a specific combined group
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }

    // Fetch combined group details from the API
    return await apiGet(`/api/active/combined/${params.id}`, token);
  },
);

/**
 * Handler for PUT /api/active/combined/[id]
 * Updates a combined group
 */
export const PUT = createPutHandler<unknown, CombinedGroupUpdateRequest>(
  async (
    _request: NextRequest,
    body: CombinedGroupUpdateRequest,
    token: string,
    params,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }

    // Update the combined group via the API
    return await apiPut(`/api/active/combined/${params.id}`, token, body);
  },
);

/**
 * Handler for DELETE /api/active/combined/[id]
 * Deletes a combined group
 */
export const DELETE = createDeleteHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }

    // Delete the combined group via the API
    await apiDelete(`/api/active/combined/${params.id}`, token);

    // Return 204 No Content response
    return null;
  },
);
