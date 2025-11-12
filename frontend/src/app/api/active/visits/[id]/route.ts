// app/api/active/visits/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiDelete, apiGet, apiPut } from "~/lib/api-helpers";
import {
  createDeleteHandler,
  createGetHandler,
  createPutHandler,
} from "~/lib/route-wrapper";

/**
 * Type definition for visit update request
 */
interface VisitUpdateRequest {
  student_id?: string;
  active_group_id?: string;
  start_time?: string;
  end_time?: string;
  // Add any other fields that can be updated
}

/**
 * Type guard to check if parameter exists and is a string
 */
function isStringParam(param: unknown): param is string {
  return typeof param === "string";
}

/**
 * Handler for GET /api/active/visits/[id]
 * Returns details of a specific visit
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }

    // Fetch visit details from the API
    return await apiGet(`/active/visits/${params.id}`, token);
  },
);

/**
 * Handler for PUT /api/active/visits/[id]
 * Updates a visit
 */
export const PUT = createPutHandler<unknown, VisitUpdateRequest>(
  async (
    _request: NextRequest,
    body: VisitUpdateRequest,
    token: string,
    params,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }

    // Update the visit via the API
    return await apiPut(`/active/visits/${params.id}`, token, body);
  },
);

/**
 * Handler for DELETE /api/active/visits/[id]
 * Deletes a visit
 */
export const DELETE = createDeleteHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }

    // Delete the visit via the API
    await apiDelete(`/active/visits/${params.id}`, token);

    // Return 204 No Content response
    return null;
  },
);
