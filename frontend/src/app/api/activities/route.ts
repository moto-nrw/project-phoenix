// app/api/activities/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";
import type { Activity, BackendActivity, CreateActivityRequest } from "~/lib/activity-helpers";
import { mapActivityResponse } from "~/lib/activity-helpers";

interface ApiResponse<T> {
  status: string;
  data: T;
}

/**
 * Handler for GET /api/activities
 * Returns a list of activities, optionally filtered by query parameters
 */
export const GET = createGetHandler(async (request: NextRequest, token: string) => {
  // Build URL with any query parameters
  const queryParams = new URLSearchParams();
  request.nextUrl.searchParams.forEach((value, key) => {
    queryParams.append(key, value);
  });
  
  const endpoint = `/api/activities${queryParams.toString() ? '?' + queryParams.toString() : ''}`;
  
  try {
    const response = await apiGet<ApiResponse<BackendActivity[]>>(endpoint, token);
    
    
    // Handle response structure
    if (response?.status === "success" && Array.isArray(response.data)) {
      return response.data.map(mapActivityResponse);
    }
    
    // If no data or unexpected structure, return empty array
    return [];
  } catch (error) {
    throw error; // Rethrow to see the real error
  }
});

/**
 * Handler for POST /api/activities
 * Creates a new activity
 */
export const POST = createPostHandler<Activity, CreateActivityRequest>(
  async (_request: NextRequest, body: CreateActivityRequest, token: string) => {
    
    // Validate required fields
    if (!body.name?.trim()) {
      throw new Error('Name is required');
    }
    if (!body.max_participants || body.max_participants <= 0) {
      throw new Error('Max participants must be greater than 0');
    }
    if (!body.category_id) {
      throw new Error('Category is required');
    }

    try {
      
      // We already have the backend data type from the request body
      const response = await apiPost<ApiResponse<BackendActivity> | BackendActivity, CreateActivityRequest>(
        `/api/activities`,
        token,
        body // Send the raw CreateActivityRequest (which already matches backend expectations)
      );
      
      
      // Create a safe activity object with default values to avoid nil pointer dereferences
      const safeActivity: Activity = {
        id: '0',
        name: body.name ?? '',
        max_participant: body.max_participants ?? 0,
        is_open_ags: false,
        supervisor_id: '',
        ag_category_id: String(body.category_id ?? ''),
        created_at: new Date(),
        updated_at: new Date(),
        participant_count: 0,
        times: [],
        students: []
      };
      
      // Try to extract data from response if possible
      if (response) {
        // Handle wrapped response { status: "success", data: BackendActivity }
        if ('status' in response && response.status === "success" && 'data' in response) {
          const backendActivity = response.data;
          if (backendActivity && 'id' in backendActivity) {
            return mapActivityResponse(backendActivity);
          }
        }
        
        // Handle direct response (BackendActivity)
        if ('id' in response) {
          return mapActivityResponse(response);
        }
        
        // Try to get ID if it exists for the safe activity fallback
        if ('id' in response) {
          safeActivity.id = String(response.id);
        } else if ('data' in response && response.data && typeof response.data === 'object' && 'id' in response.data) {
          safeActivity.id = String(response.data.id);
        }
        
        // Return the safe activity with as much data as we could extract
        return safeActivity;
      }
      
      // If we get here, the request was successful but we couldn't parse the response
      // Just return the safe activity with the data from the request
      return safeActivity;
    } catch (error) {
      throw error; // Rethrow to see the real error
    }
  }
);