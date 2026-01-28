// app/api/activities/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";
import type {
  Activity,
  BackendActivity,
  CreateActivityRequest,
} from "~/lib/activity-helpers";
import { mapActivityResponse } from "~/lib/activity-helpers";

interface ApiResponse<T> {
  status: string;
  data: T;
}

interface BackendResponse {
  status?: string;
  data?: BackendActivity[];
  message?: string;
}

/**
 * Handler for GET /api/activities
 * Returns a list of activities, optionally filtered by query parameters
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    // Build URL with any query parameters
    const queryParams = new URLSearchParams();
    request.nextUrl.searchParams.forEach((value, key) => {
      queryParams.append(key, value);
    });

    // Override page_size to load all activities at once for frontend search
    queryParams.set("page_size", "1000");

    const endpoint = `/api/activities${queryParams.toString() ? "?" + queryParams.toString() : ""}`;

    try {
      const response = await apiGet<BackendResponse>(endpoint, token);

      // Check for status: "success" wrapper (backend response format)
      if (
        response?.status === "success" &&
        response?.data &&
        Array.isArray(response.data)
      ) {
        const mappedData = response.data.map(mapActivityResponse);

        return {
          data: mappedData,
          pagination: {
            current_page: 1,
            page_size: mappedData.length,
            total_pages: 1,
            total_records: mappedData.length,
          },
        };
      }

      // Check if response has data property with activities array
      if (response?.data && Array.isArray(response.data)) {
        const mappedData = response.data.map(mapActivityResponse);

        return {
          data: mappedData,
          pagination: {
            current_page: 1,
            page_size: mappedData.length,
            total_pages: 1,
            total_records: mappedData.length,
          },
        };
      }

      // Check if response is directly an array of activities
      if (Array.isArray(response)) {
        const mappedData = (response as unknown as BackendActivity[]).map(
          mapActivityResponse,
        );

        return {
          data: mappedData,
          pagination: {
            current_page: 1,
            page_size: mappedData.length,
            total_pages: 1,
            total_records: mappedData.length,
          },
        };
      }

      // If no data or unexpected structure, return empty paginated response
      return {
        data: [],
        pagination: {
          current_page: 1,
          page_size: 50,
          total_pages: 0,
          total_records: 0,
        },
      };
    } catch (error) {
      console.error("Error in activities route:", error);
      throw error; // Rethrow to see the real error
    }
  },
);

/** Extract BackendActivity from various response formats */
function extractBackendActivity(
  response: ApiResponse<BackendActivity> | BackendActivity | null,
): BackendActivity | null {
  if (!response) return null;

  // Handle wrapped response { status: "success", data: BackendActivity }
  if (
    "status" in response &&
    response.status === "success" &&
    "data" in response
  ) {
    const data = response.data;
    if (data && "id" in data) return data;
  }

  // Handle direct response (BackendActivity)
  if ("id" in response) return response;

  return null;
}

/** Create fallback activity from request body */
function createFallbackActivity(body: CreateActivityRequest): Activity {
  return {
    id: "0",
    name: body.name ?? "",
    max_participant: body.max_participants ?? 0,
    is_open_ags: false,
    supervisor_id: "",
    ag_category_id: String(body.category_id ?? ""),
    created_at: new Date(),
    updated_at: new Date(),
    participant_count: 0,
    times: [],
    students: [],
  };
}

/**
 * Handler for POST /api/activities
 * Creates a new activity
 */
export const POST = createPostHandler<Activity, CreateActivityRequest>(
  async (_request: NextRequest, body: CreateActivityRequest, token: string) => {
    // Validate required fields
    if (!body.name?.trim()) throw new Error("Name is required");
    if (!body.max_participants || body.max_participants <= 0) {
      throw new Error("Max participants must be greater than 0");
    }
    if (!body.category_id) throw new Error("Category is required");

    const response = await apiPost<
      ApiResponse<BackendActivity> | BackendActivity,
      CreateActivityRequest
    >(`/api/activities`, token, body);

    // Try to extract and map the backend activity
    const backendActivity = extractBackendActivity(response);
    if (backendActivity) return mapActivityResponse(backendActivity);

    // Fallback: return safe activity from request data
    return createFallbackActivity(body);
  },
);
