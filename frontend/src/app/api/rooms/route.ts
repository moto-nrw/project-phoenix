// app/api/rooms/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";
import type { BackendRoom } from "~/lib/room-helpers";

/**
 * Type definition for room creation request
 */
interface RoomCreateRequest {
  name: string;
  building?: string;
  floor: number;
  capacity: number;
  category: string;
  color: string;
  device_id?: string;
}

/**
 * Type definition for API response format
 */
interface ApiRoomsResponse {
  status: string;
  data: BackendRoomResponse[];
}

/**
 * Partial backend room response type to handle the raw API data
 */
interface BackendRoomResponse {
  id: number;
  name: string;
  room_name?: string;
  building?: string;
  floor: number;
  capacity: number;
  category: string;
  color: string;
  device_id?: string;
  is_occupied: boolean;
  created_at: string;
  updated_at: string;
}

/**
 * Handler for GET /api/rooms
 * Returns a list of rooms, optionally filtered by query parameters
 */
export const GET = createGetHandler(async (request: NextRequest, token: string) => {
  // Build URL with any query parameters
  const queryParams = new URLSearchParams();
  request.nextUrl.searchParams.forEach((value, key) => {
    queryParams.append(key, value);
  });
  
  const endpoint = `/api/rooms${queryParams.toString() ? '?' + queryParams.toString() : ''}`;
  
  try {
    // Fetch rooms from backend API
    const response = await apiGet<ApiRoomsResponse>(endpoint, token);
    
    // Handle null or undefined response
    if (!response) {
      console.warn("API returned null response for rooms");
      return [];
    }
    
    // Debug output to check the response data
    console.log("API rooms response:", JSON.stringify(response));
    
    // The response has a nested structure with the rooms in the data field
    if (response.status === "success" && Array.isArray(response.data)) {
      // Map the response data to ensure consistent field names
      const mappedRooms = response.data.map((room: BackendRoomResponse) => ({
        ...room,
        // Ensure all required fields exist
        id: String(room.id), // Convert to string to match frontend expectations
        name: room.name ?? room.room_name ?? "",
        isOccupied: room.is_occupied ?? false,
        capacity: room.capacity ?? 0,
        category: room.category ?? "Other",
        color: room.color ?? "#FFFFFF",
        deviceId: room.device_id ?? "",
        createdAt: room.created_at ?? "",
        updatedAt: room.updated_at ?? ""
      }));
      
      return mappedRooms;
    }
    
    // If the response doesn't have the expected structure, return an empty array
    console.warn("API response does not have the expected structure:", response);
    return [];
  } catch (error) {
    console.error("Error fetching rooms:", error);
    // Return empty array instead of throwing error
    return [];
  }
});

/**
 * Handler for POST /api/rooms
 * Creates a new room
 */
export const POST = createPostHandler<BackendRoom, RoomCreateRequest>(
  async (_request: NextRequest, body: RoomCreateRequest, token: string) => {
    // Validate required fields
    if (!body.name || body.name.trim() === '') {
      throw new Error('Missing required field: name cannot be blank');
    }
    if (body.capacity === undefined || body.capacity <= 0) {
      throw new Error('Capacity must be greater than 0');
    }
    if (!body.category) {
      throw new Error('Missing required field: category');
    }
    
    try {
      // Create the room via the API
      return await apiPost<BackendRoom>("/api/rooms", token, body);
    } catch (error) {
      // Check for permission errors (403 Forbidden)
      if (error instanceof Error && error.message.includes("403")) {
        console.error("Permission denied when creating room:", error);
        throw new Error("Permission denied: You need the 'rooms:create' permission to create rooms.");
      }
      
      // Check for validation errors 
      if (error instanceof Error && error.message.includes("400")) {
        const errorMessage = error.message;
        console.error("Validation error when creating room:", errorMessage);
        
        // Extract specific error message if possible
        if (errorMessage.includes("name: cannot be blank")) {
          throw new Error("Room name cannot be blank");
        }
      }
      
      // Re-throw other errors
      throw error;
    }
  }
);