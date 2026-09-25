// app/api/rooms/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";
import type { BackendRoom } from "~/lib/room-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "RoomsRoute" });

/**
 * Type definition for room creation request
 */
interface RoomCreateRequest {
  name: string;
  building?: string;
  floor?: number; // Optional
  capacity?: number | null; // Optional
  category?: string; // Optional
  color?: string; // Optional
  device_id?: string;
}

/**
 * Type definition for API response format
 */
interface ApiRoomsResponse {
  status: string;
  data: BackendRoomResponse[];
  pagination?: {
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  };
  message?: string;
}

/**
 * Partial backend room response type to handle the raw API data
 */
interface BackendRoomResponse {
  id: number;
  name: string;
  room_name?: string;
  building?: string;
  floor?: number | null; // Optional (nullable in DB)
  capacity?: number | null; // Optional (nullable in DB)
  category?: string | null; // Optional (nullable in DB)
  color?: string | null; // Optional (nullable in DB)
  device_id?: string;
  is_occupied: boolean;
  group_name?: string;
  category_name?: string;
  activity_name?: string;
  supervisor_name?: string;
  student_count?: number;
  created_at: string;
  updated_at: string;
}

/**
 * Handler for GET /api/rooms
 * Returns a list of rooms, optionally filtered by query parameters
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    // Build URL with any query parameters
    const queryParams = new URLSearchParams();
    request.nextUrl.searchParams.forEach((value, key) => {
      queryParams.append(key, value);
    });

    const endpoint = `/api/rooms${queryParams.toString() ? "?" + queryParams.toString() : ""}`;

    try {
      // Fetch rooms from backend API
      const response = await apiGet<ApiRoomsResponse>(endpoint, token);

      // Handle null or undefined response
      if (!response) {
        logger.warn("API returned null response for rooms");
        return [];
      }

      logger.debug("rooms API response received");

      // The response has a nested structure with the rooms in the data field
      if (response.status === "success" && Array.isArray(response.data)) {
        // Keep the original backend format since the service factory will handle mapping
        return {
          data: response.data,
          pagination: response.pagination,
          status: response.status,
          message: response.message,
        };
      }

      logger.warn("rooms API response has unexpected structure");
      return {
        data: [],
        pagination: {
          current_page: 1,
          page_size: 50,
          total_pages: 1,
          total_records: 0,
        },
      };
    } catch (error) {
      logger.warn("rooms fetch failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },
);

/**
 * Handler for POST /api/rooms
 * Creates a new room
 */
export const POST = createPostHandler<BackendRoom, RoomCreateRequest>(
  async (_request: NextRequest, body: RoomCreateRequest, token: string) => {
    // Validate required fields - only name is mandatory
    if (!body.name || body.name.trim() === "") {
      throw new Error("Missing required field: name cannot be blank");
    }

    // Validate capacity if provided (must be > 0)
    if (body.capacity != null && body.capacity <= 0) {
      throw new Error("Capacity must be greater than 0");
    }

    return apiPost<BackendRoom>("/api/rooms", token, body);
  },
);
