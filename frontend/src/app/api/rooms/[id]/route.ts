// app/api/rooms/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers";
import { createGetHandler, createPutHandler, createDeleteHandler } from "~/lib/route-wrapper";
import type { BackendRoom } from "~/lib/room-helpers";

/**
 * Type definition for room update request
 * Accommodates both camelCase (frontend) and snake_case (backend) field names
 */
interface RoomUpdateRequest {
  // Frontend form fields (camelCase)
  name?: string;
  building?: string;
  floor?: number;
  capacity?: number;
  category?: string;
  color?: string;
  deviceId?: string;
  
  // Backend fields (snake_case) - for compatibility
  device_id?: string;
}

/**
 * Interface for the backend room response object
 */
interface BackendRoomResponse {
  data?: {
    id: number;
    name: string;
    building?: string;
    floor: number;
    capacity: number;
    category: string;
    color: string;
    device_id?: string;
    is_occupied: boolean;
    activity_name?: string;
    group_name?: string;
    supervisor_name?: string;
    student_count?: number;
    created_at: string;
    updated_at: string;
  };
  id?: number;
  name?: string;
  building?: string;
  floor?: number;
  capacity?: number;
  category?: string;
  color?: string;
  device_id?: string;
  is_occupied?: boolean;
  activity_name?: string;
  group_name?: string;
  supervisor_name?: string;
  student_count?: number;
  created_at?: string;
  updated_at?: string;
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
    const response = await apiGet<BackendRoomResponse>(`/api/rooms/${params.id}`, token);
    
    // Check if the response is in the expected format
    console.log("Room API response:", JSON.stringify(response));
    
    // Handle different response formats
    if (response && typeof response === 'object') {
      if ('data' in response && typeof response.data === 'object') {
        // Wrapped response format - normalize field names
        const roomData = response.data;
        return {
          id: roomData.id,
          name: roomData.name,
          building: roomData.building,
          floor: roomData.floor,
          capacity: roomData.capacity,
          category: roomData.category,
          color: roomData.color,
          device_id: roomData.device_id,
          is_occupied: roomData.is_occupied ?? false,
          activity_name: roomData.activity_name,
          group_name: roomData.group_name,
          supervisor_name: roomData.supervisor_name,
          student_count: roomData.student_count,
          created_at: roomData.created_at,
          updated_at: roomData.updated_at
        };
      } else if ('id' in response) {
        // Direct room object - ensure all fields use snake_case
        return {
          id: response.id,
          name: response.name, 
          building: response.building,
          floor: response.floor,
          capacity: response.capacity,
          category: response.category,
          color: response.color,
          device_id: response.device_id,
          is_occupied: response.is_occupied ?? false,
          activity_name: response.activity_name,
          group_name: response.group_name,
          supervisor_name: response.supervisor_name,
          student_count: response.student_count,
          created_at: response.created_at,
          updated_at: response.updated_at
        };
      }
    }
    
    // If format is unexpected, throw error
    console.error("Unexpected room response format:", response);
    throw new Error("Unexpected room response format");
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
      // Prepare payload for backend API
      // Map frontend form fields to backend field names
      // The backend API expects snake_case while frontend uses camelCase
      const backendBody = {
        name: body.name,
        building: body.building,
        floor: body.floor,
        capacity: body.capacity,
        category: body.category,
        color: body.color,
        // Handle deviceId (camelCase from frontend) to device_id (snake_case for backend)
        device_id: body.device_id ?? body.deviceId
      };
      
      // Update the room via the API and get the updated room data
      const updatedRoom = await apiPut<BackendRoom>(`/api/rooms/${params.id}`, token, backendBody);
      
      console.log("Room updated successfully:", updatedRoom);
      
      // Make sure we return a properly formatted response with all fields
      // Return the result directly as a BackendRoom
      return updatedRoom;
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