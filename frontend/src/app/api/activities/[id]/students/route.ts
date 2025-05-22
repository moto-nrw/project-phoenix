// app/api/activities/[id]/students/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";
import { 
  getEnrolledStudents,
  updateGroupEnrollments,
  getAvailableStudents,
  enrollStudent
} from "~/lib/activity-api";
import type { BackendStudentEnrollment } from "~/lib/activity-helpers";
import { mapStudentEnrollmentsResponse } from "~/lib/activity-helpers";

/**
 * Handler for GET /api/activities/[id]/students
 * Returns a list of students enrolled in a specific activity
 */
export const GET = createGetHandler(async (request: NextRequest, token: string, params: Record<string, unknown>) => {
  const id = params.id as string;
  
  // Check for "available" query parameter - return available students instead of enrolled ones
  const available = request.nextUrl.searchParams.get("available") === "true";
  
  if (available) {
    // Get query parameters for filtering
    const search = request.nextUrl.searchParams.get("search") ?? undefined;
    const groupId = request.nextUrl.searchParams.get("group_id") ?? undefined;
    
    // Get students available for enrollment
    const availableStudents = await getAvailableStudents(id, { 
      search, 
      group_id: groupId 
    });
    
    return availableStudents;
  }
  
  // Otherwise return enrolled students - call backend directly
  try {
    const endpoint = `/api/activities/${id}/students`;
    const response = await apiGet<{ data: BackendStudentEnrollment[] }>(endpoint, token);
    const enrollments = response.data ?? [];
    // Map the backend enrollment structure to frontend format
    return mapStudentEnrollmentsResponse(enrollments);
  } catch {
    return []; // Return empty array on error
  }
});

/**
 * Handler for POST /api/activities/[id]/students
 * Supports two modes:
 * 1. Regular: Enrolls a single student - expects { student_id: string }
 * 2. Batch: Updates multiple students at once - expects { student_ids: string[] }
 */
export const POST = createPostHandler(
  async (request: NextRequest, body: { student_ids?: string[]; student_id?: string }, token: string, params: Record<string, unknown>) => {
    const id = params.id as string;
    
    // Check if this is a batch update operation
    if (body.student_ids && Array.isArray(body.student_ids)) {
      try {
        // Use batch update function
        const success = await updateGroupEnrollments(id, { 
          student_ids: body.student_ids 
        });
        
        if (success) {
          // Return updated enrolled students
          const students = await getEnrolledStudents(id);
          return students;
        } else {
          throw new Error("Failed to update enrollments");
        }
      } catch (error) {
        throw error;
      }
    } 
    // Regular single student enrollment
    else if (body.student_id) {
      try {
        const studentId = String(body.student_id);
        // Enroll a single student
        const result = await enrollStudent(id, { studentId });
        
        if (result.success) {
          // Return updated enrolled students
          const students = await getEnrolledStudents(id);
          return students; 
        } else {
          throw new Error("Failed to enroll student");
        }
      } catch (error) {
        throw error;
      }
    }
    else {
      throw new Error("Invalid request: must provide student_id or student_ids");
    }
  }
);