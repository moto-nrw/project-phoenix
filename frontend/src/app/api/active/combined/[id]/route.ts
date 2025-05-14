// app/api/active/combined/[id]/route.ts
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { apiDelete, apiGet, apiPut } from "~/lib/api-helpers";
import { createDeleteHandler, createGetHandler, createPutHandler } from "~/lib/route-wrapper";

/**
 * Type definition for combined group update request
 */
interface CombinedGroupUpdateRequest {
  name?: string;
  description?: string;
  room_id?: string;
}

/**
 * Handler for GET /api/active/combined/[id]
 * Returns details of a specific combined group
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string, params) => {
  const id = params.id as string;
  
  // Fetch combined group details from the API
  return await apiGet(`/active/combined/${id}`, token);
});

/**
 * Handler for PUT /api/active/combined/[id]
 * Updates a combined group
 */
export const PUT = createPutHandler<unknown, CombinedGroupUpdateRequest>(
  async (_request: NextRequest, body: CombinedGroupUpdateRequest, token: string, params) => {
    const id = params.id as string;
    
    // Update the combined group via the API
    return await apiPut(`/active/combined/${id}`, token, body);
  }
);

/**
 * Handler for DELETE /api/active/combined/[id]
 * Deletes a combined group
 */
export const DELETE = createDeleteHandler(async (_request: NextRequest, token: string, params) => {
  const id = params.id as string;
  
  // Delete the combined group via the API
  await apiDelete(`/active/combined/${id}`, token);
  
  // Return 204 No Content response
  return null;
});