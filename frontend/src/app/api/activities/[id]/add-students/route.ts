// app/api/activities/[id]/add-students/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";
import type { BackendActivityStudent, BackendActivity } from "~/lib/activity-helpers";

/**
 * Handler for GET /api/activities/[id]/add-students
 * Returns a list of eligible students that can be added to the activity
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string, params: Record<string, unknown>) => {
  const id = params.id as string;
  const endpoint = `/api/activities/${id}/eligible-students`;
  
  try {
    const response = await apiGet<{ data: any[]; status: string } | any[]>(endpoint, token);
    
    // Handle response structure
    if (response && 'status' in response && response.status === "success" && 'data' in response && Array.isArray(response.data)) {
      return response.data;
    } else if (Array.isArray(response)) {
      return response;
    }
    
    // If no data or unexpected structure, return empty array
    console.log('Unexpected response structure:', response);
    return [];
  } catch (error) {
    console.error('Error fetching eligible students:', error);
    return []; // Return empty array on error
  }
});

/**
 * Handler for POST /api/activities/[id]/add-students
 * Adds multiple students to the activity in a batch
 */
export const POST = createPostHandler<{ success: boolean; count: number }, { student_ids: (string | number)[] }>(
  async (_request: NextRequest, body: { student_ids: (string | number)[] }, token: string, params: Record<string, unknown>) => {
    const id = params.id as string;
    const endpoint = `/api/activities/${id}/students/batch`;
    
    // Ensure all student_ids are numbers for the backend
    const backendData = {
      student_ids: body.student_ids.map(studentId => 
        typeof studentId === 'string' ? parseInt(studentId, 10) : studentId
      )
    };
    
    try {
      const response = await apiPost<any, { student_ids: number[] }>(endpoint, token, backendData);
      
      // If we have a specific count in the response, use it
      if (response && typeof response === 'object' && 'count' in response) {
        return { 
          success: true,
          count: response.count
        };
      }
      
      // Otherwise just return generic success with the count we sent
      return { 
        success: true,
        count: backendData.student_ids.length
      };
    } catch (error) {
      console.error('Error batch enrolling students:', error);
      throw error;
    }
  }
);