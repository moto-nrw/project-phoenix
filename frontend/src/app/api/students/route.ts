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
 * Creates a new student without requiring user account or RFID
 */
export const POST = createPostHandler<Student, CreateStudentRequest>(
  async (_request: NextRequest, body: CreateStudentRequest, token: string) => {
    // Validate required fields
    if (!body.first_name || body.first_name.trim() === '') {
      throw new Error('Missing required field: first_name cannot be blank');
    }
    
    if (!body.school_class || body.school_class.trim() === '') {
      throw new Error('Missing required field: school_class cannot be blank');
    }
    
    // Transform the request to match backend expectations
    const backendRequest = {
      student: {
        first_name: body.first_name,
        last_name: body.second_name ?? body.first_name, // Backend requires last_name
        tag_id: body.tag_id, // Don't send empty string - let backend handle the logic
        school_class: body.school_class,
        guardian_name: body.name_lg ?? "Unknown",
        guardian_contact: body.contact_lg ?? "Unknown",
        guardian_email: body.guardian_email ?? (body.contact_lg?.includes('@') ? body.contact_lg : undefined),
        guardian_phone: body.guardian_phone ?? (body.contact_lg?.includes('@') ? undefined : body.contact_lg),
        group_id: body.group_id ? parseInt(body.group_id.toString(), 10) : undefined,
        account_id: body.account_id, // Pass the account ID if provided
      },
      email: body.email ?? (body.contact_lg?.includes('@') ? body.contact_lg : `student${Date.now()}@example.com`),
      password: body.password ?? "default123456", // Use provided password or generate default
    };
    
    try {
      // Create the student via the new API endpoint that doesn't require user creation
      const response = await apiPost<StudentResponseFromBackend>("/api/students", token, backendRequest.student);
      
      // Map the backend response to frontend format
      return {
        id: String(response.id),
        name: `${response.first_name} ${response.last_name}`,
        first_name: response.first_name,
        second_name: response.last_name,
        school_class: response.school_class,
        grade: undefined,
        studentId: response.tag_id,
        group_name: undefined,
        group_id: response.group_id ? String(response.group_id) : undefined,
        in_house: response.location === "In House",
        wc: response.location === "WC",
        school_yard: response.location === "School Yard",
        bus: response.location === "Bus",
        name_lg: response.guardian_name,
        contact_lg: response.guardian_contact,
        custom_users_id: undefined,
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
        if (errorMessage.includes("first_name: cannot be blank")) {
          throw new Error("First name cannot be blank");
        }
        if (errorMessage.includes("school_class: cannot be blank")) {
          throw new Error("School class cannot be blank");
        }
      }
      
      // Re-throw other errors
      throw error;
    }
  }
);