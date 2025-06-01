// app/api/students/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers";
import { createGetHandler, createPutHandler, createDeleteHandler } from "~/lib/route-wrapper";
import type { BackendStudent, Student } from "~/lib/student-helpers";
import { mapStudentResponse, prepareStudentForBackend } from "~/lib/student-helpers";

/**
 * Type definition for API response format
 * Backend wraps response in { status: "success", data: {...}, message: "..." }
 */
interface ApiStudentResponse {
  status: string;
  data: StudentResponseFromBackend;
  message: string;
}

/**
 * Type definition for student response from backend
 */
interface StudentResponseFromBackend {
  id: number;
  person_id: number;
  first_name: string;
  last_name: string;
  tag_id?: string;
  school_class: string;
  location: string;
  guardian_name: string;
  guardian_contact: string;
  guardian_email?: string;
  guardian_phone?: string;
  group_id?: number;
  created_at: string;
  updated_at: string;
}

/**
 * Handler for GET /api/students/[id]
 * Returns a single student by ID
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string, params: Record<string, unknown>) => {
  const id = params.id as string;
  
  if (!id) {
    throw new Error('Student ID is required');
  }
  
  try {
    // Fetch student from backend API
    // Using unknown type and will validate structure
    const response = await apiGet<unknown>(`/api/students/${id}`, token);
    
    // Type guard to check response structure
    if (!response || typeof response !== 'object' || !('data' in response)) {
      console.warn("API returned invalid response for student");
      throw new Error('Student not found');
    }
    
    const typedResponse = response as { data: unknown };
    
    // Map the backend response to frontend format
    const studentData = typedResponse.data as BackendStudent;
    return mapStudentResponse(studentData);
  } catch (error) {
    console.error("Error fetching student:", error);
    throw error;
  }
});

/**
 * Handler for PUT /api/students/[id]
 * Updates an existing student
 */
export const PUT = createPutHandler<Student, Partial<Student>>(
  async (_request: NextRequest, body: Partial<Student>, token: string, params: Record<string, unknown>) => {
    const id = params.id as string;
    
    if (!id) {
      throw new Error('Student ID is required');
    }
    
    try {
      // Transform frontend format to backend format
      const backendData = prepareStudentForBackend(body);
      
      // Call backend API to update student
      const response = await apiPut<ApiStudentResponse>(
        `/api/students/${id}`,
        token,
        backendData
      );
      
      // Handle null or undefined response
      if (!response?.data) {
        throw new Error('Invalid response from backend');
      }
      
      // Map the response to frontend format
      const mappedStudent = mapStudentResponse(response.data as BackendStudent);
      return mappedStudent;
    } catch (error) {
      console.error("Error updating student:", error);
      throw error;
    }
  }
);

/**
 * Handler for DELETE /api/students/[id]
 * Deletes a student
 */
export const DELETE = createDeleteHandler(async (_request: NextRequest, token: string, params: Record<string, unknown>) => {
  const id = params.id as string;
  
  if (!id) {
    throw new Error('Student ID is required');
  }
  
  try {
    // Call backend API to delete student
    const response = await apiDelete<{ message: string }>(
      `/api/students/${id}`,
      token
    );
    
    return { success: true, message: response?.message ?? 'Student deleted successfully' };
  } catch (error) {
    console.error("Error deleting student:", error);
    throw error;
  }
});