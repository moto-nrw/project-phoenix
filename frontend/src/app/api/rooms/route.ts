import { NextResponse, type NextRequest } from "next/server";
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

export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const queryParams = new URLSearchParams();
    request.nextUrl.searchParams.forEach((value, key) => {
      queryParams.append(key, value);
    });

    const endpoint = `/api/rooms${queryParams.toString() ? "?" + queryParams.toString() : ""}`;

    try {
      const response = await apiGet<ApiRoomsResponse>(endpoint, token);

      if (!response) {
        logger.warn("API returned null response for rooms");
        return NextResponse.json([]);
      }

      logger.debug("rooms API response received");

      if (response.status === "success" && Array.isArray(response.data)) {
        // The backend format as it is; the service factory maps it.
        return NextResponse.json({
          data: response.data,
          pagination: response.pagination,
          status: response.status,
          message: response.message,
        });
      }

      logger.warn("rooms API response has unexpected structure");
      return NextResponse.json({
        data: [],
        pagination: {
          current_page: 1,
          page_size: 50,
          total_pages: 1,
          total_records: 0,
        },
      });
    } catch (error) {
      logger.warn("rooms fetch failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },
);

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
