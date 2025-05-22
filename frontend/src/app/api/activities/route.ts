// app/api/activities/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";
import type { Activity, BackendActivity, CreateActivityRequest } from "~/lib/activity-helpers";
import { mapActivityResponse } from "~/lib/activity-helpers";

// Mock data for testing
const MOCK_ACTIVITIES: BackendActivity[] = [
  {
    id: 1,
    name: "Fußball AG",
    max_participants: 20,
    is_open: true,
    category_id: 1,
    supervisor_ids: [1],
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  {
    id: 2,
    name: "Chor",
    max_participants: 30,
    is_open: true,
    category_id: 2,
    supervisor_ids: [2],
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
];

let MOCK_ID_COUNTER = 3;

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
    const response = await apiGet<{ status: string; data: BackendActivity[] }>(endpoint, token);
    
    console.log('Activities fetch response:', response);
    
    // Handle response structure
    if (response && response.status === "success" && Array.isArray(response.data)) {
      return response.data.map(mapActivityResponse);
    }
    
    // If no data or unexpected structure, return empty array
    console.log('Unexpected response structure:', response);
    return [];
  } catch (error) {
    console.error('Error fetching activities:', error);
    throw error; // Rethrow to see the real error
  }
});

/**
 * Handler for POST /api/activities
 * Creates a new activity
 */
export const POST = createPostHandler<Activity, CreateActivityRequest>(
  async (_request: NextRequest, body: CreateActivityRequest, token: string) => {
    console.log('Frontend API Route - Received request body:', JSON.stringify(body, null, 2));
    console.log('Frontend API Route - Schedules data:', body.schedules);
    
    // Validate required fields
    if (!body.name || body.name.trim() === '') {
      throw new Error('Name is required');
    }
    if (!body.max_participants || body.max_participants <= 0) {
      throw new Error('Max participants must be greater than 0');
    }
    if (!body.category_id) {
      throw new Error('Category is required');
    }

    try {
      console.log('Frontend API Route - Sending to backend:', JSON.stringify(body, null, 2));
      
      // We already have the backend data type from the request body
      const response = await apiPost<any, CreateActivityRequest>(
        `/api/activities`,
        token,
        body // Send the raw CreateActivityRequest (which already matches backend expectations)
      );
      
      console.log('Activity creation response:', response);
      
      // Create a safe activity object with default values to avoid nil pointer dereferences
      const safeActivity: Activity = {
        id: '0',
        name: body.name || '',
        max_participant: body.max_participants || 0,
        is_open_ags: false,
        supervisor_id: '',
        ag_category_id: String(body.category_id || ''),
        created_at: new Date(),
        updated_at: new Date(),
        participant_count: 0,
        times: [],
        students: []
      };
      
      // Try to extract data from response if possible
      if (response) {
        // Handle wrapped response { status: "success", data: BackendActivity }
        if (typeof response === 'object' && 'status' in response && response.status === "success" && 'data' in response) {
          const backendActivity = response.data;
          if (backendActivity && typeof backendActivity === 'object' && 'id' in backendActivity) {
            return mapActivityResponse(backendActivity as BackendActivity);
          }
        } 
        // Handle direct response (BackendActivity)
        else if (typeof response === 'object' && 'id' in response) {
          return mapActivityResponse(response as BackendActivity);
        }
      }
      
      // If we couldn't properly extract data but the POST was successful,
      // return a placeholder activity with any data we can extract 
      if (response) {
        // Try to get ID if it exists
        if (typeof response === 'object' && 'id' in response) {
          safeActivity.id = String(response.id);
        } else if (typeof response === 'object' && 'data' in response && 
                  typeof response.data === 'object' && 'id' in response.data) {
          safeActivity.id = String(response.data.id);
        }
        
        // Return the safe activity with as much data as we could extract
        return safeActivity;
      }
      
      // If we get here, the request was successful but we couldn't parse the response
      // Just return the safe activity with the data from the request
      return safeActivity;
    } catch (error) {
      console.error('Error creating activity:', error);
      throw error; // Rethrow to see the real error
    }
  }
);