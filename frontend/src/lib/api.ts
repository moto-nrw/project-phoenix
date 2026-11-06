import {
  ApiError,
  apiErrorFromBody,
  apiErrorFromText,
  transportFetch,
} from "./api-error";
import { clearSessionCache, getCachedSession } from "./session-cache";
import { createLogger } from "~/lib/logger";
import api from "./api-transport";
import { resolveApiUrl } from "./api-url";
import { convertToBackendRoom, fetchWithRetry } from "./api-helpers";
import {
  mapSingleGroupResponse,
  mapGroupResponse, // Used internally in getGroup
  prepareGroupForBackend,
  mapGroupsResponse,
} from "./group-helpers";

export { mapGroupResponse } from "./group-helpers";
import type { BackendGroup, Group as ImportedGroup } from "./group-helpers";
import {
  mapSingleRoomResponse,
  prepareRoomForBackend,
  mapRoomsResponse,
} from "./room-helpers";

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
 * A failed browser call as an ApiError: status, code, field errors and
 * request ID from the envelope, the `detail` as diagnostic message, and the
 * raw body for callers that read extension members (companion-plan 409).
 */
function browserApiError(status: number, body: string): Error {
  const error = apiErrorFromText(`API error: ${status}`, status, body);
  Object.assign(error, { body });
  return error;
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
 * Extract single BackendRoom from various response formats.
 * Handles: wrapped {data: BackendRoom}, direct BackendRoom with id.
 */
function extractBackendRoom(responseData: unknown): BackendRoom {
  if (!responseData || typeof responseData !== "object") {
    throw new Error("Unexpected room response format");
  }

  const data = responseData as Record<string, unknown>;

  // Format 1: Wrapped { data: BackendRoom }
  if ("data" in data && data.data) {
    return data.data as BackendRoom;
  }

  // Format 2: Direct BackendRoom (has 'id' property)
  if ("id" in data) {
    return convertToBackendRoom(data);
  }

  logger.warn("unexpected room response format", {
    response_type: typeof responseData,
  });
  throw new Error("Unexpected room response format");
}

/**
 * Validate room data before creation.
 * Throws descriptive error if validation fails.
 */
function validateRoomForCreation(room: {
  name?: string;
  capacity?: number | null;
  category?: string;
}): void {
  if (!room.name) {
    throw new Error("Missing required field: name");
  }
  if (
    room.capacity !== undefined &&
    room.capacity !== null &&
    room.capacity <= 0
  ) {
    throw new Error("capacity must be greater than 0");
  }
  if (!room.category) {
    throw new Error("Missing required field: category");
  }
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

/**
 * Parse single group response, extracting BackendGroup from various wrapper formats.
 * Handles: ApiResponse wrapper, data wrapper, double-wrapped, and direct formats.
 */
function extractBackendGroup(responseData: unknown): BackendGroup {
  if (!responseData || typeof responseData !== "object") {
    throw new Error("Invalid response format from API");
  }

  const data = responseData as Record<string, unknown>;

  // Format 1: ApiResponse { success: true, data: BackendGroup | { data: BackendGroup } }
  if ("success" in data && "data" in data) {
    const innerData = data.data;
    // Check for double-wrapped { data: { data: BackendGroup } }
    if (
      innerData &&
      typeof innerData === "object" &&
      "data" in (innerData as Record<string, unknown>)
    ) {
      return (innerData as { data: BackendGroup }).data;
    }
    return innerData as BackendGroup;
  }

  // Format 2: Simple wrapper { data: BackendGroup }
  if ("data" in data) {
    return data.data as BackendGroup;
  }

  // Format 3: Direct BackendGroup (has 'id' and 'name' properties)
  if ("id" in data && "name" in data) {
    return data as unknown as BackendGroup;
  }

  throw new Error("No group data in response");
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
    const baseUrl = await resolveApiUrl("/api/groups");
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

  // Get a specific group by ID
  getGroup: async (id: string): Promise<Group> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = await resolveApiUrl(`/api/groups/${id}`);

    try {
      if (useProxyApi) {
        // Browser environment: use fetchWithRetry for automatic 401 handling
        const { response, data, accessDeniedStatus, accessDeniedBody } =
          await fetchWithRetry<unknown>(url, {
            onAuthFailure: handleAuthFailure,
            beforeRetry: reloadSession,
          });

        if (response === null) {
          throw accessDeniedApiError(accessDeniedStatus, accessDeniedBody);
        }

        const groupData = extractBackendGroup(data);
        return mapGroupResponse(groupData);
      }

      // Server-side: use axios with the API URL directly
      const response = await api.get(url);
      return mapGroupResponse(response.data as BackendGroup);
    } catch (error) {
      logger.error("failed to fetch group", {
        group_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Create a new group
  createGroup: async (group: Omit<Group, "id">): Promise<Group> => {
    // Transform from frontend model to backend model
    const backendGroup = prepareGroupForBackend(group);

    // Basic validation for group creation
    if (!backendGroup.name) {
      throw new Error("Missing required field: name");
    }

    const useProxyApi = globalThis.window !== undefined;
    const url = await resolveApiUrl("/api/groups");

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
          method: "POST",
          credentials: "include",
          headers: JSON_HEADERS,
          body: JSON.stringify(backendGroup),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });
          throw browserApiError(response.status, errorText);
        }

        const data = (await response.json()) as BackendGroup;
        return mapSingleGroupResponse({ data });
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.post(url, backendGroup);
        return mapSingleGroupResponse({ data: response.data as BackendGroup });
      }
    } catch (error) {
      logger.error("failed to create group", {
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Update a group
  updateGroup: async (id: string, group: Partial<Group>): Promise<Group> => {
    // Transform from frontend model to backend model updates
    const backendUpdates = prepareGroupForBackend(group);

    const useProxyApi = globalThis.window !== undefined;
    const url = await resolveApiUrl(`/api/groups/${id}`);

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
          method: "PUT",
          credentials: "include",
          headers: JSON_HEADERS,
          body: JSON.stringify(backendUpdates),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });

          // Every other refusal keeps code, field errors and request ID for
          // the shared error path (#2513).
          throw browserApiError(response.status, errorText);
        }

        const data = (await response.json()) as BackendGroup;
        return mapSingleGroupResponse({ data });
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.put(url, backendUpdates);
        return mapSingleGroupResponse({ data: response.data as BackendGroup });
      }
    } catch (error) {
      logger.error("failed to update group", {
        group_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Delete a group
  deleteGroup: async (id: string): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = await resolveApiUrl(`/api/groups/${id}`);

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
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
          throw browserApiError(response.status, errorText);
        }
        return;
      }

      // Server-side: use axios with the API URL directly
      await api.delete(url);
    } catch (error) {
      logger.error("failed to delete group", {
        group_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Add a supervisor to a group
  addSupervisor: async (
    groupId: string,
    supervisorId: string,
  ): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = await resolveApiUrl(`/api/groups/${groupId}/supervisors`);

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
          method: "POST",
          credentials: "include",
          headers: JSON_HEADERS,
          body: JSON.stringify({
            supervisor_id: Number.parseInt(supervisorId, 10),
          }),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });
          throw browserApiError(response.status, errorText);
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        await api.post(url, {
          supervisor_id: Number.parseInt(supervisorId, 10),
        });
        return;
      }
    } catch (error) {
      logger.error("failed to add supervisor to group", {
        supervisor_id: supervisorId,
        group_id: groupId,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Remove a supervisor from a group
  removeSupervisor: async (
    groupId: string,
    supervisorId: string,
  ): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = await resolveApiUrl(
      `/api/groups/${groupId}/supervisors/${supervisorId}`,
    );

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
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
          throw browserApiError(response.status, errorText);
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        await api.delete(url);
        return;
      }
    } catch (error) {
      logger.error("failed to remove supervisor from group", {
        supervisor_id: supervisorId,
        group_id: groupId,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Set the representative for a group
  setRepresentative: async (
    groupId: string,
    representativeId: string,
  ): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = await resolveApiUrl(`/api/groups/${groupId}/representative`);

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
          method: "PUT",
          credentials: "include",
          headers: JSON_HEADERS,
          body: JSON.stringify({
            representative_id: Number.parseInt(representativeId, 10),
          }),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });
          throw browserApiError(response.status, errorText);
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        await api.put(url, {
          representative_id: Number.parseInt(representativeId, 10),
        });
        return;
      }
    } catch (error) {
      logger.error("failed to set representative for group", {
        representative_id: representativeId,
        group_id: groupId,
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
    const baseUrl = await resolveApiUrl("/api/rooms");
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

  // Get a specific room by ID
  getRoom: async (id: string): Promise<Room> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = await resolveApiUrl(`/api/rooms/${id}`);

    try {
      if (useProxyApi) {
        // Browser environment: use fetchWithRetry for automatic 401 handling
        const { response, data, accessDeniedStatus, accessDeniedBody } =
          await fetchWithRetry<unknown>(url, {
            onAuthFailure: handleAuthFailure,
            beforeRetry: reloadSession,
          });

        if (response === null) {
          throw accessDeniedApiError(accessDeniedStatus, accessDeniedBody);
        }

        const roomData = extractBackendRoom(data);
        return mapSingleRoomResponse({ data: roomData });
      }

      // Server-side: use axios with the API URL directly
      const response = await api.get(url);
      const roomData = extractBackendRoom(response.data);
      return mapSingleRoomResponse({ data: roomData });
    } catch (error) {
      logger.error("failed to fetch room", {
        room_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Create a new room
  createRoom: async (room: Omit<Room, "id" | "isOccupied">): Promise<Room> => {
    // Validate room data before transformation
    validateRoomForCreation(room);

    // Transform from frontend model to backend model
    const backendRoom = prepareRoomForBackend(room);

    const useProxyApi = globalThis.window !== undefined;
    const url = await resolveApiUrl("/api/rooms");

    try {
      if (useProxyApi) {
        const response = await fetch(url, {
          method: "POST",
          credentials: "include",
          headers: JSON_HEADERS,
          body: JSON.stringify(backendRoom),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });
          throw browserApiError(response.status, errorText);
        }

        const data = (await response.json()) as BackendRoom;
        return mapSingleRoomResponse({ data });
      }

      // Server-side: use axios with the API URL directly
      const response = await api.post(url, backendRoom);
      return mapSingleRoomResponse({ data: response.data as BackendRoom });
    } catch (error) {
      logger.error("failed to create room", {
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Update a room
  updateRoom: async (id: string, room: Partial<Room>): Promise<Room> => {
    // Transform from frontend model to backend model updates
    const backendUpdates = prepareRoomForBackend(room);

    const useProxyApi = globalThis.window !== undefined;
    const url = await resolveApiUrl(`/api/rooms/${id}`);

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
          method: "PUT",
          credentials: "include",
          headers: JSON_HEADERS,
          body: JSON.stringify(backendUpdates),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });

          // Every other refusal keeps code, field errors and request ID for
          // the shared error path (#2513).
          throw browserApiError(response.status, errorText);
        }

        const data = (await response.json()) as BackendRoom;
        return mapSingleRoomResponse({ data });
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.put(url, backendUpdates);
        return mapSingleRoomResponse({ data: response.data as BackendRoom });
      }
    } catch (error) {
      logger.error("failed to update room", {
        room_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Delete a room
  deleteRoom: async (id: string): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = await resolveApiUrl(`/api/rooms/${id}`);

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

  // Get rooms grouped by category
  getRoomsByCategory: async (): Promise<Record<string, Room[]>> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = await resolveApiUrl("/api/rooms/by-category");

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
          credentials: "include",
          headers: JSON_HEADERS,
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });
          throw browserApiError(response.status, errorText);
        }

        const data = (await response.json()) as Record<string, BackendRoom[]>;

        // Transform each category's room array
        const result: Record<string, Room[]> = {};
        for (const [category, rooms] of Object.entries(data)) {
          result[category] = mapRoomsResponse(rooms);
        }

        return result;
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        const data = response.data as Record<string, BackendRoom[]>;

        // Transform each category's room array
        const result: Record<string, Room[]> = {};
        for (const [category, rooms] of Object.entries(data)) {
          result[category] = mapRoomsResponse(rooms);
        }

        return result;
      }
    } catch (error) {
      logger.error("failed to fetch rooms by category", {
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },
};

export default api;
