// app/api/activities/[id]/schedules/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";
import type { BackendActivitySchedule } from "~/lib/activity-helpers";
import { mapActivityScheduleResponse } from "~/lib/activity-helpers";

/**
 * Handler for GET /api/activities/[id]/schedules
 * Returns a list of schedules for a specific activity
 */
export const GET = createGetHandler(async (request: NextRequest, token: string, params: Record<string, unknown>) => {
  const id = params.id as string;
  const endpoint = `/api/activities/${id}/schedules`;
  
  try {
    const response = await apiGet<{ data: BackendActivitySchedule[]; status: string } | BackendActivitySchedule[]>(endpoint, token);
    
    // Handle response structure
    if (response && 'status' in response && response.status === "success" && 'data' in response && Array.isArray(response.data)) {
      // Handle wrapped response { status: "success", data: BackendActivitySchedule[] }
      return response.data.map(mapActivityScheduleResponse);
    } else if (Array.isArray(response)) {
      // Handle direct response (BackendActivitySchedule[])
      return response.map(mapActivityScheduleResponse);
    }
    
    // If we get here, we have a response but it's not in the expected format
    throw new Error(`Unexpected response structure from schedules API for activity ${id}`);
  } catch (error) {
    
    // Properly propagate the error for handling in the service layer
    throw new Error(`Failed to fetch schedules for activity ${id}: ${error instanceof Error ? error.message : 'Unknown error'}`);
  }
});

/**
 * Handler for POST /api/activities/[id]/schedules
 * Creates a new schedule for a specific activity
 */
export const POST = createPostHandler<BackendActivitySchedule, { weekday: string; timeframe_id?: number }>(
  async (request: NextRequest, body: { weekday: string; timeframe_id?: number }, token: string, params: Record<string, unknown>): Promise<BackendActivitySchedule> => {
    const id = params.id as string;
    const endpoint = `/api/activities/${id}/schedules`;
    
    
    try {
      const response = await apiPost<{ status: string; data: BackendActivitySchedule } | BackendActivitySchedule, { weekday: string; timeframe_id?: number }>(endpoint, token, body);
      
      
      // Handle wrapped response { status: "success", data: BackendActivitySchedule }
      if (response && typeof response === 'object' && 'status' in response && response.status === "success" && 'data' in response) {
        return response.data;
      }
      
      // Handle direct response (BackendActivitySchedule)
      if (response && typeof response === 'object' && 'id' in response) {
        return response;
      }
      
      throw new Error('Unexpected response structure from schedule creation API');
    } catch (error) {
      throw error;
    }
  }
);