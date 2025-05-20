// app/api/activities/[id]/students/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost, apiDelete } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler, createDeleteHandler } from "~/lib/route-wrapper";
import type { BackendActivityStudent } from "~/lib/activity-helpers";

/**
 * Handler for GET /api/activities/[id]/students
 * Returns a list of students enrolled in a specific activity
 */
export const GET = createGetHandler(async (request: NextRequest, token: string, params: Record<string, unknown>) => {
  const id = params.id as string;
  const endpoint = `/api/activities/${id}/students`;
  
  try {
    const response = await apiGet<{ data: BackendActivityStudent[]; status: string } | BackendActivityStudent[]>(endpoint, token);
    
    console.log('Students fetch response:', response);
    
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
    console.error('Error fetching enrolled students:', error);
    return []; // Return empty array on error
  }
});

/**
 * Handler for POST /api/activities/[id]/students
 * Enrolls a student in the activity
 */
export const POST = createPostHandler<{ success: boolean }, { student_id: string | number }>(
  async (_request: NextRequest, body: { student_id: string | number }, token: string, params: Record<string, unknown>) => {
    const id = params.id as string;
    const endpoint = `/api/activities/${id}/students`;
    
    // Ensure student_id is a number for the backend
    const backendData = {
      student_id: typeof body.student_id === 'string' ? parseInt(body.student_id, 10) : body.student_id
    };
    
    try {
      await apiPost(endpoint, token, backendData);
      return { success: true };
    } catch (error) {
      console.error('Error enrolling student:', error);
      throw error;
    }
  }
);

/**
 * Handler for DELETE /api/activities/[id]/students/[studentId]
 * Unenrolls a student from the activity
 */
export const DELETE = createDeleteHandler(
  async (_request: NextRequest, token: string, params: Record<string, unknown>) => {
    const id = params.id as string;
    const studentId = params.studentId as string;
    const endpoint = `/api/activities/${id}/students/${studentId}`;
    
    try {
      await apiDelete(endpoint, token);
      return { success: true };
    } catch (error) {
      console.error('Error unenrolling student:', error);
      throw error;
    }
  }
);