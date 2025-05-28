import type { NextRequest } from "next/server";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers";
import { createGetHandler, createPutHandler, createDeleteHandler } from "~/lib/route-wrapper";
import type { BackendGroup } from "~/lib/group-helpers";

/**
 * Type definition for group update request
 */
interface GroupUpdateRequest {
  name?: string;
  description?: string;
  room_id?: number;
  representative_id?: number;
  teacher_ids?: number[];
}

/**
 * Handler for GET /api/groups/[id]
 * Returns a specific group by ID
 */
export const GET = createGetHandler(async (request: NextRequest, token: string, params: Record<string, unknown>) => {
  const id = params.id as string;
  const endpoint = `/api/groups/${id}`;
  
  // Fetch group from the API
  const response = await apiGet<BackendGroup>(endpoint, token);
  console.log('Backend API response:', response);
  
  // If response is undefined or null, throw an error
  if (!response) {
    throw new Error('Group not found');
  }
  
  // The response is already a BackendGroup, not wrapped
  // Return it directly so that createGetHandler can wrap it once
  return response;
});

/**
 * Handler for PUT /api/groups/[id]
 * Updates a specific group by ID
 */
export const PUT = createPutHandler(async (request: NextRequest, body: GroupUpdateRequest, token: string, params: Record<string, unknown>) => {
  const id = params.id as string;
  const endpoint = `/api/groups/${id}`;
  
  // Update group via the API
  return await apiPut<BackendGroup>(endpoint, token, body);
});

/**
 * Handler for DELETE /api/groups/[id]
 * Deletes a specific group by ID
 */
export const DELETE = createDeleteHandler(async (request: NextRequest, token: string, params: Record<string, unknown>) => {
  const id = params.id as string;
  const endpoint = `/api/groups/${id}`;
  
  // Delete group via the API
  return await apiDelete(endpoint, token);
});