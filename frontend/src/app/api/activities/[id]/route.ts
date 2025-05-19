// app/api/activities/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers";
import { createGetHandler, createPutHandler, createDeleteHandler } from "~/lib/route-wrapper";
import type { Activity, BackendActivity, UpdateActivityRequest } from "~/lib/activity-helpers";
import { mapActivityResponse } from "~/lib/activity-helpers";

// Import mock data from parent route
const MOCK_ACTIVITIES: BackendActivity[] = [
  {
    id: 1,
    name: "Fußball AG",
    max_participants: 20,
    category_id: 1,
    supervisor_ids: [1],
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  {
    id: 2,
    name: "Chor",
    max_participants: 30,
    category_id: 2,
    supervisor_ids: [2],
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
];

/**
 * Handler for GET /api/activities/[id]
 * Returns a single activity by ID
 */
export const GET = createGetHandler(async (request: NextRequest, token: string, params: Record<string, unknown>) => {
  const id = params.id as string;
  
  try {
    const response = await apiGet<BackendActivity | { status: string; data: BackendActivity }>(`/api/activities/${id}`, token);
    
    // Handle both response formats (raw object or wrapped in status/data)
    if (response) {
      if ('status' in response && response.status === "success" && 'data' in response) {
        // Handle wrapped response { status: "success", data: BackendActivity }
        return mapActivityResponse(response.data);
      } else if ('id' in response) {
        // Handle direct response (BackendActivity)
        return mapActivityResponse(response as BackendActivity);
      }
    }
    
    throw new Error('Unexpected response structure');
  } catch (error) {
    console.log('Error fetching activity:', error);
    
    // If the error contains a 404 status, return the appropriate error
    if (error instanceof Error && error.message.includes('API error (404)')) {
      throw new Error(`API error (404): Activity with ID ${id} not found`);
    }
    
    // No more mock data fallback - throw the error to show proper errors
    throw error;
  }
});

/**
 * Handler for PUT /api/activities/[id]
 * Updates an existing activity
 */
export const PUT = createPutHandler<Activity, UpdateActivityRequest>(
  async (_request: NextRequest, body: UpdateActivityRequest, token: string, params: Record<string, unknown>) => {
    const id = params.id as string;
    const activityId = parseInt(id);

    if (isNaN(activityId)) {
      throw new Error('Invalid activity ID');
    }

    try {
      // The body already matches the UpdateActivityRequest structure expected by backend
      const response = await apiPut<BackendActivity | { status: string; data: BackendActivity }, UpdateActivityRequest>(
        `/api/activities/${id}`,
        token,
        body // Send the raw UpdateActivityRequest
      );
      
      // Handle both response formats (raw object or wrapped in status/data)
      if (response) {
        if ('status' in response && response.status === "success" && 'data' in response) {
          // Handle wrapped response { status: "success", data: BackendActivity }
          return mapActivityResponse(response.data);
        } else if ('id' in response) {
          // Handle direct response (BackendActivity)
          return mapActivityResponse(response as BackendActivity);
        }
      }
      
      throw new Error('Unexpected response structure');
    } catch (error) {
      console.log('Error updating activity:', error);
      // Don't use mock data - throw the real error
      throw error;
    }
  }
);

/**
 * Handler for DELETE /api/activities/[id]
 * Removes an activity by ID
 */
export const DELETE = createDeleteHandler(async (request: NextRequest, token: string, params: Record<string, unknown>) => {
  const id = params.id as string;
  const activityId = parseInt(id);

  if (isNaN(activityId)) {
    throw new Error('Invalid activity ID');
  }

  try {
    const response = await apiDelete<{ status?: string | number; success?: boolean }>(`/api/activities/${id}`, token);
    
    // Backend typically returns no content on successful delete
    if (!response || (response.status && response.status === 204) || (response.status && response.status === '204') || response.success) {
      return { success: true };
    }
    
    // Also handle the case where status might be "success"
    if (response.status && response.status === "success") {
      return { success: true };
    }
    
    throw new Error('Unexpected response structure');
  } catch (error) {
    console.log('Error deleting activity:', error);
    // Don't use mock delete - throw the real error
    throw error;
  }
});