import {
  ApiError,
  apiErrorFromBody,
  apiErrorFromText,
  transportFetch,
} from "./api-error";
import { clearSessionCache, getCachedSession } from "./session-cache";
import { createLogger } from "~/lib/logger";
import api from "./api-transport";
import { fetchWithRetry } from "./api-helpers";
import { mapGroupsResponse } from "./group-helpers";

import type { BackendGroup, Group as ImportedGroup } from "./group-helpers";
import { mapRoomsResponse } from "./room-helpers";

import type { BackendRoom } from "./room-helpers";
import { handleAuthFailure } from "./auth-failure";

// Logger instance for API client
const logger = createLogger({ component: "ApiClient" });

// Paginated response interface for API responses with pagination metadata
interface PaginatedResponse<T> {
  status: string;
  data: T[];
  pagination: {
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  };
  message?: string;
}

/**
 * Headers for the raw fetch() call sites in the services below. The BFF
 * routes authenticate through the session cookie; the browser holds no
 * token (#5005).
 */
const JSON_HEADERS: Record<string, string> = {
  "Content-Type": "application/json",
};

/**
 * Re-read the session after a 401 → refresh (helper for fetchWithRetry).
 * The cached session is stale by definition — drop it first, or this (and
 * every concurrent caller for up to 10s) would skip the lookup that persists
 * the rotated cookie before the retry.
 */
async function reloadSession(): Promise<void> {
  clearSessionCache();
  await getCachedSession();
}

/**
 * The refusal fetchWithRetry reported, with code, details and request ID of
 * the backend envelope when its body carried one (ADR 0006).
 */
function accessDeniedApiError(status?: 401 | 403, body?: unknown): ApiError {
  const accessDeniedStatus = status ?? 401;
  return apiErrorFromBody(
    accessDeniedStatus === 403 ? "Access denied" : "Authentication failed",
    accessDeniedStatus,
    body,
  );
}

/**
 * Build query parameters for room filters.
 */
function buildRoomQueryParams(filters?: {
  building?: string;
  floor?: number;
  category?: string;
  occupied?: boolean;
  search?: string;
  includeSystem?: boolean;
  page?: number;
  pageSize?: number;
}): URLSearchParams {
  const params = new URLSearchParams();
  if (filters?.search) params.append("search", filters.search);
  if (filters?.building) params.append("building", filters.building);
  if (filters?.floor !== undefined)
    params.append("floor", filters.floor.toString());
  if (filters?.category) params.append("category", filters.category);
  if (filters?.occupied !== undefined)
    params.append("occupied", filters.occupied.toString());
  if (filters?.page !== undefined)
    params.append("page", filters.page.toString());
  if (filters?.pageSize !== undefined)
    params.append("page_size", filters.pageSize.toString());
  if (filters?.includeSystem !== undefined)
    params.append("include_system", String(filters.includeSystem));
  return params;
}

/**
 * Parse rooms response, handling null, non-array, and valid array formats.
 * Returns empty array for invalid formats with warning.
 */
function parseRoomsResponse(responseData: unknown): BackendRoom[] {
  if (
    responseData &&
    typeof responseData === "object" &&
    "data" in responseData &&
    Array.isArray((responseData as { data?: unknown }).data)
  ) {
    return (responseData as { data: BackendRoom[] }).data;
  }

  if (!responseData || !Array.isArray(responseData)) {
    logger.warn("invalid response format for rooms", {
      response_type: typeof responseData,
      is_array: Array.isArray(responseData),
    });
    return [];
  }
  return responseData as BackendRoom[];
}

/**
 * Parse groups response from API.
 * Handles wrapped {data: BackendGroup[]} and direct BackendGroup[] formats.
 */
function parseGroupsResponse(responseData: unknown): BackendGroup[] {
  // Check if wrapped in ApiResponse format {data: [...]}
  if (
    typeof responseData === "object" &&
    responseData !== null &&
    "data" in responseData
  ) {
    const apiResponse = responseData as { data?: unknown };
    return Array.isArray(apiResponse.data)
      ? (apiResponse.data as BackendGroup[])
      : [];
  }

  // Direct array response
  if (Array.isArray(responseData)) {
    return responseData as BackendGroup[];
  }

  return [];
}

// Re-export types for external usage
export type { Student } from "./student-helpers";
export type Group = ImportedGroup;

// Room-related interfaces
export interface Room {
  id: string;
  name: string;
  building?: string;
  floor?: number; // Optional (nullable in DB)
  capacity?: number; // Optional (nullable in DB)
  category?: string; // Optional (nullable in DB)
  color?: string; // Optional (nullable in DB)
  deviceId?: string;
  isOccupied: boolean;
  activityName?: string;
  groupName?: string;
  supervisorName?: string;
  studentCount?: number;
  createdAt?: string;
  updatedAt?: string;
}

// Group service for API operations
export const groupService = {
  // Get all groups
  getGroups: async (
    filters?: { search?: string; page?: number; pageSize?: number },
    options?: { strict?: boolean },
  ): Promise<Group[]> => {
    const params = new URLSearchParams();
    if (filters?.search) params.append("search", filters.search);

    if (filters?.page !== undefined) params.set("page", String(filters.page));
    if (filters?.pageSize !== undefined)
      params.set("page_size", String(filters.pageSize));

    const useProxyApi = globalThis.window !== undefined;
    const queryString = params.toString();
    const baseUrl = "/api/groups";
    const url = queryString ? `${baseUrl}?${queryString}` : baseUrl;

    try {
      if (useProxyApi) {
        // Browser environment: use fetchWithRetry for automatic 401 handling
        const { response, data, accessDeniedStatus, accessDeniedBody } =
          await fetchWithRetry<unknown>(url, {
            onAuthFailure: handleAuthFailure,
            beforeRetry: reloadSession,
          });

        // Handle errors: null response means auth failed or permission denied
        // Return empty array for graceful degradation
        if (response === null || data === null) {
          if (options?.strict)
            throw accessDeniedApiError(accessDeniedStatus, accessDeniedBody);
          return [];
        }

        return mapGroupsResponse(parseGroupsResponse(data));
      }

      // Server-side: use axios with the API URL directly
      const response = await api.get(url, { params });
      const paginatedResponse =
        response.data as PaginatedResponse<BackendGroup>;
      return mapGroupsResponse(paginatedResponse.data);
    } catch (error) {
      logger.error("failed to fetch groups", {
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },
};

// Room service for API operations
export const roomService = {
  // Get all rooms
  getRooms: async (
    filters?: {
      building?: string;
      floor?: number;
      category?: string;
      occupied?: boolean;
      search?: string;
      includeSystem?: boolean;
      page?: number;
      pageSize?: number;
    },
    options?: { strict?: boolean },
  ): Promise<Room[]> => {
    const params = buildRoomQueryParams(filters);
    const queryString = params.toString();

    const useProxyApi = globalThis.window !== undefined;
    const baseUrl = "/api/rooms";
    const url = queryString ? `${baseUrl}?${queryString}` : baseUrl;

    try {
      if (useProxyApi) {
        // Browser environment: use fetchWithRetry for automatic 401 handling
        const { response, data, accessDeniedStatus, accessDeniedBody } =
          await fetchWithRetry<unknown>(url, {
            onAuthFailure: handleAuthFailure,
            beforeRetry: reloadSession,
          });

        if (options?.strict && (response === null || data === null)) {
          throw accessDeniedApiError(accessDeniedStatus, accessDeniedBody);
        }

        const rooms = parseRoomsResponse(data);
        return mapRoomsResponse(rooms);
      }

      // Server-side: use axios with the API URL directly
      const response = await api.get(url, { params });
      const rooms = parseRoomsResponse(response.data);
      return mapRoomsResponse(rooms);
    } catch (error) {
      logger.error("failed to fetch rooms", {
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Delete a room
  deleteRoom: async (id: string): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = `/api/rooms/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await transportFetch(url, {
          method: "DELETE",
          credentials: "include",
          headers: JSON_HEADERS,
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });
          // Code, Feldfehler und Vorgangskennung bleiben erhalten (#2517).
          throw apiErrorFromText(
            `API error: ${response.status}`,
            response.status,
            errorText,
          );
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        await api.delete(url);
        return;
      }
    } catch (error) {
      logger.error("failed to delete room", {
        room_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },
};

export default api;
