// app/api/students/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers";
import { createGetHandler, createPutHandler, createDeleteHandler } from "~/lib/route-wrapper";
import type { BackendStudent, Student, UpdateStudentRequest } from "~/lib/student-helpers";
import { mapStudentResponse } from "~/lib/student-helpers";

/**
 * Type definition for API response format
 */
interface ApiStudentResponse {
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
    const response = await apiGet<ApiStudentResponse>(`/api/students/${id}`, token);
    
    // Handle null or undefined response
    if (!response?.data) {
      console.warn("API returned null response for student");
      throw new Error('Student not found');
    }
    
    const student = response.data;
    
    // Map the response to frontend format using the same mapping function for consistency
    const mappedStudent = mapStudentResponse(student as BackendStudent);
    return mappedStudent;
  } catch (error) {
    console.error("Error fetching student:", error);
    throw error;
  }
});

/**
 * Handler for PUT /api/students/[id]
 * Updates an existing student (currently not supported by backend)
 */
export const PUT = createPutHandler<Student, UpdateStudentRequest>(
  async (_request: NextRequest, body: UpdateStudentRequest, token: string, params: Record<string, unknown>) => {
    const id = params.id as string;
    
    if (!id) {
      throw new Error('Student ID is required');
    }
    
    // Backend doesn't support updating students yet
    throw new Error('Updating students is not currently supported by the backend API');
  }
);

/**
 * Handler for DELETE /api/students/[id]
 * Deletes a student (currently not supported by backend)
 */
export const DELETE = createDeleteHandler(async (_request: NextRequest, token: string, params: Record<string, unknown>) => {
  const id = params.id as string;
  
  if (!id) {
    throw new Error('Student ID is required');
  }
  
  // Backend doesn't support deleting students yet
  throw new Error('Deleting students is not currently supported by the backend API');
});