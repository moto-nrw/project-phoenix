// app/api/rooms/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers";
import { createGetHandler, createPutHandler, createDeleteHandler } from "~/lib/route-wrapper";
import type { BackendRoom } from "~/lib/room-helpers";

/**
 * Type definition for room update request
 */
interface RoomUpdateRequest {
  room_name?: string;
  building?: string;
  floor?: number;
  capacity?: number;
  category?: string;
  color?: string;
  device_id?: string;
}

/**
 * Type guard to check if parameter exists and is a string
 */
function isStringParam(param: unknown): param is string {
  return typeof param === 'string';
}

/**
 * Handler for GET /api/rooms/[id]
 * Returns details of a specific room
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string, params) => {
  if (!isStringParam(params.id)) {
    throw new Error('Invalid id parameter');
  }
  
  try {
    // Call the backend API to get the room data
    return await apiGet<BackendRoom>(`/api/rooms/${params.id}`, token);
  } catch (error) {
    console.error(`Error fetching room ${params.id}:`, error);
    // If we get a 404 or database error, return a properly formatted error
    if (error instanceof Error && 
        (error.message.includes('404') || 
         error.message.includes('relation "rooms" does not exist'))) {
      throw new Error(`Room with ID ${params.id} not found`);
    }
    // Re-throw other errors
    throw error;
  }
});

/**
 * Handler for PUT /api/rooms/[id]
 * Updates a room
 */
export const PUT = createPutHandler<BackendRoom, RoomUpdateRequest>(
  async (_request: NextRequest, body: RoomUpdateRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error('Invalid id parameter');
    }
    
    // Validate update data if provided
    if (body.capacity !== undefined && body.capacity <= 0) {
      throw new Error('Capacity must be greater than 0');
    }
    
    try {
      // Update the room via the API
      return await apiPut<BackendRoom>(`/api/rooms/${params.id}`, token, body);
    } catch (error) {
      console.error(`Error updating room ${params.id}:`, error);
      // If we get a 404 or database error, return a properly formatted error
      if (error instanceof Error && 
          (error.message.includes('404') || 
           error.message.includes('relation "rooms" does not exist'))) {
        throw new Error(`Room with ID ${params.id} not found`);
      }
      // Re-throw other errors
      throw error;
    }
  }
);

/**
 * Handler for DELETE /api/rooms/[id]
 * Deletes a room
 */
export const DELETE = createDeleteHandler(async (_request: NextRequest, token: string, params) => {
  if (!isStringParam(params.id)) {
    throw new Error('Invalid id parameter');
  }
  
  try {
    // Delete the room via the API
    await apiDelete(`/api/rooms/${params.id}`, token);
    
    // Return 204 No Content response
    return null;
  } catch (error) {
    console.error(`Error deleting room ${params.id}:`, error);
    // If we get a 404 or database error, return a properly formatted error
    if (error instanceof Error && 
        (error.message.includes('404') || 
         error.message.includes('relation "rooms" does not exist'))) {
      throw new Error(`Room with ID ${params.id} not found`);
    }
    // Re-throw other errors
    throw error;
  }
});