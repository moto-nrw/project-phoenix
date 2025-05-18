// app/api/students/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";
import type { Student, CreateStudentRequest } from "~/lib/student-helpers";
import { mapStudentResponse } from "~/lib/student-helpers";

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
 * Type definition for API response format from backend
 */
interface ApiStudentsResponse {
  status: string;
  data: StudentResponseFromBackend[];
  pagination: {
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  };
  message: string;
}

/**
 * Handler for GET /api/students
 * Returns a list of students, optionally filtered by query parameters
 */
export const GET = createGetHandler(async (request: NextRequest, token: string): Promise<Student[]> => {
  // Build URL with any query parameters
  const queryParams = new URLSearchParams();
  request.nextUrl.searchParams.forEach((value, key) => {
    queryParams.append(key, value);
  });
  
  const endpoint = `/api/students${queryParams.toString() ? '?' + queryParams.toString() : ''}`;
  
  console.log("Requesting students from backend:", endpoint);
  
  try {
    // Fetch students from backend API
    const response = await apiGet<ApiStudentsResponse>(endpoint, token);
    
    // Handle null or undefined response
    if (!response) {
      console.warn("API returned null response for students");
      return [];
    }
    
    // Debug output to check the response data
    console.log("API students response:", JSON.stringify(response, null, 2));
    
    // Check for the paginated response structure from backend
    if ('data' in response && Array.isArray(response.data)) {
      // If data is empty, return empty array
      if (response.data.length === 0) {
        return [];
      }
      
      // Map the backend response format to the frontend format
      const mappedStudents = response.data.map((student: StudentResponseFromBackend): Student => ({
        id: String(student.id),
        name: `${student.first_name} ${student.last_name}`,
        first_name: student.first_name,
        second_name: student.last_name,
        school_class: student.school_class,
        grade: undefined, // Not provided by backend
        studentId: student.tag_id,
        group_name: undefined, // Not provided directly, needs to be looked up if needed
        group_id: student.group_id ? String(student.group_id) : undefined,
        in_house: student.location === "In House",
        wc: student.location === "WC",
        school_yard: student.location === "School Yard",
        bus: student.location === "Bus",
        name_lg: student.guardian_name,
        contact_lg: student.guardian_contact,
        custom_users_id: undefined, // Not provided by backend
      }));
      
      return mappedStudents;
    }
    
    // If the response doesn't have the expected structure, return an empty array
    console.warn("API response does not have the expected structure:", response);
    return [];
  } catch (error) {
    console.error("Error fetching students:", error);
    // Log the specific error for debugging
    if (error instanceof Error) {
      console.error("Error message:", error.message);
      console.error("Error stack:", error.stack);
    }
    throw error; // Re-throw to let the error handler deal with it
  }
});

/**
 * Handler for POST /api/students
 * Creates a new student with associated person record
 */
export const POST = createPostHandler<Student, any>(
  async (_request: NextRequest, body: any, token: string) => {
    // Body is already in backend format from prepareStudentForBackend
    // Validate required fields using backend field names
    if (!body.first_name || body.first_name.trim() === '') {
      throw new Error('First name is required');
    }
    
    if (!body.last_name || body.last_name.trim() === '') {
      throw new Error('Last name is required');
    }
    
    if (!body.school_class || body.school_class.trim() === '') {
      throw new Error('School class is required');
    }
    
    if (!body.guardian_name || body.guardian_name.trim() === '') {
      throw new Error('Guardian name is required');
    }
    
    if (!body.guardian_contact || body.guardian_contact.trim() === '') {
      throw new Error('Guardian contact is required');
    }
    
    // No transformation needed - body is already in backend format
    const backendRequest = body;
    
    try {
      // Create the student via the simplified API endpoint
      const response = await apiPost<StudentResponseFromBackend>("/api/students", token, backendRequest);
      
      // Map the backend response to frontend format
      return {
        id: String(response.id),
        name: `${response.first_name} ${response.last_name}`,
        first_name: response.first_name,
        second_name: response.last_name,
        school_class: response.school_class,
        grade: undefined, // Not used by backend
        studentId: response.tag_id,
        group_name: undefined, // Would need separate lookup
        group_id: response.group_id ? String(response.group_id) : undefined,
        in_house: response.location === "In House",
        wc: response.location === "WC",
        school_yard: response.location === "School Yard",
        bus: response.location === "Bus",
        name_lg: response.guardian_name,
        contact_lg: response.guardian_contact,
        custom_users_id: undefined, // Not used by backend
      };
    } catch (error) {
      // Check for permission errors (403 Forbidden)
      if (error instanceof Error && error.message.includes("403")) {
        console.error("Permission denied when creating student:", error);
        throw new Error("Permission denied: You need the 'users:create' permission to create students.");
      }
      
      // Check for validation errors 
      if (error instanceof Error && error.message.includes("400")) {
        const errorMessage = error.message;
        console.error("Validation error when creating student:", errorMessage);
        
        // Extract specific error message if possible
        if (errorMessage.includes("first name is required")) {
          throw new Error("First name is required");
        }
        if (errorMessage.includes("school class is required")) {
          throw new Error("School class is required");
        }
        if (errorMessage.includes("guardian name is required")) {
          throw new Error("Guardian name is required");
        }
        if (errorMessage.includes("guardian contact is required")) {
          throw new Error("Guardian contact is required");
        }
      }
      
      // Re-throw other errors
      throw error;
    }
  }
);