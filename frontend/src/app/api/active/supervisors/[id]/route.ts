// app/api/active/supervisors/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiDelete, apiGet, apiPut } from "~/lib/api-helpers";
import { createDeleteHandler, createGetHandler, createPutHandler } from "~/lib/route-wrapper";

/**
 * Type definition for supervisor update request
 */
interface SupervisorUpdateRequest {
  staff_id?: string;
  active_group_id?: string;
  // Add any other fields that can be updated
}

/**
 * Type guard to check if parameter exists and is a string
 */
function isStringParam(param: unknown): param is string {
  return typeof param === 'string';
}

/**
 * Handler for GET /api/active/supervisors/[id]
 * Returns details of a specific supervisor
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string, params) => {
  if (!isStringParam(params.id)) {
    throw new Error('Invalid id parameter');
  }
  
  // Fetch supervisor details from the API
  return await apiGet(`/active/supervisors/${params.id}`, token);
});

/**
 * Handler for PUT /api/active/supervisors/[id]
 * Updates a supervisor
 */
export const PUT = createPutHandler<unknown, SupervisorUpdateRequest>(
  async (_request: NextRequest, body: SupervisorUpdateRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error('Invalid id parameter');
    }
    
    // Update the supervisor via the API
    return await apiPut(`/active/supervisors/${params.id}`, token, body);
  }
);

/**
 * Handler for DELETE /api/active/supervisors/[id]
 * Deletes a supervisor
 */
export const DELETE = createDeleteHandler(async (_request: NextRequest, token: string, params) => {
  if (!isStringParam(params.id)) {
    throw new Error('Invalid id parameter');
  }
  
  // Delete the supervisor via the API
  await apiDelete(`/active/supervisors/${params.id}`, token);
  
  // Return 204 No Content response
  return null;
});