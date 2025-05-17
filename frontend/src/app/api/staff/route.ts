// app/api/staff/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";

/**
 * Type definition for staff member response from backend
 */
interface BackendStaffResponse {
  id: number;
  person_id: number;
  staff_notes?: string;
  is_teacher: boolean;
  teacher_id?: number;
  specialization?: string;
  role?: string;
  qualifications?: string;
  person?: {
    id: number;
    first_name: string;
    last_name: string;
    email?: string;
    tag_id?: string;
    created_at: string;
    updated_at: string;
  };
  created_at: string;
  updated_at: string;
}

/**
 * Type definition for staff creation request
 */
interface StaffCreateRequest {
  person_id: number;
  staff_notes?: string;
  is_teacher?: boolean;
  specialization?: string;
  role?: string;
  qualifications?: string;
}

/**
 * Type definition for API response format
 */
interface ApiStaffResponse {
  status: string;
  data: BackendStaffResponse[];
}

/**
 * Handler for GET /api/staff
 * Returns a list of staff members, optionally filtered by query parameters
 */
export const GET = createGetHandler(async (request: NextRequest, token: string) => {
  // Build URL with any query parameters
  const queryParams = new URLSearchParams();
  request.nextUrl.searchParams.forEach((value, key) => {
    queryParams.append(key, value);
  });
  
  const endpoint = `/api/staff${queryParams.toString() ? '?' + queryParams.toString() : ''}`;
  
  console.log("Requesting staff from backend:", endpoint);
  
  try {
    // Fetch staff from backend API
    const response = await apiGet<BackendStaffResponse[] | ApiStaffResponse>(endpoint, token);
    
    // Handle null or undefined response
    if (!response) {
      console.warn("API returned null response for staff");
      return [];
    }
    
    // Debug output to check the response data
    console.log("API staff response:", JSON.stringify(response, null, 2));
    
    // Check if the response is already an array (common pattern)
    if (Array.isArray(response)) {
      // Direct array response - filter and map
      const mappedStaff = response
        .filter((staff: BackendStaffResponse) => staff.is_teacher) // Only include teachers
        .map((staff: BackendStaffResponse) => ({
          id: String(staff.id), // Convert to string to match frontend expectations
          name: staff.person ? `${staff.person.first_name} ${staff.person.last_name}` : "",
          first_name: staff.person?.first_name ?? "",
          last_name: staff.person?.last_name ?? "",
          specialization: staff.specialization ?? "",
          role: staff.role ?? null,
          qualifications: staff.qualifications ?? null,
          tag_id: staff.person?.tag_id ?? null,
          staff_notes: staff.staff_notes ?? null,
          created_at: staff.created_at,
          updated_at: staff.updated_at,
        }));
      
      return mappedStaff;
    }
    
    // Check for nested data structure
    if ('data' in response && Array.isArray(response.data)) {
      // Map the response data to match the Teacher interface from teacher-api.ts
      const mappedStaff = response.data
        .filter((staff: BackendStaffResponse) => staff.is_teacher) // Only include teachers
        .map((staff: BackendStaffResponse) => ({
          id: String(staff.id), // Convert to string to match frontend expectations
          name: staff.person ? `${staff.person.first_name} ${staff.person.last_name}` : "",
          first_name: staff.person?.first_name ?? "",
          last_name: staff.person?.last_name ?? "",
          specialization: staff.specialization ?? "",
          role: staff.role ?? null,
          qualifications: staff.qualifications ?? null,
          tag_id: staff.person?.tag_id ?? null,
          staff_notes: staff.staff_notes ?? null,
          created_at: staff.created_at,
          updated_at: staff.updated_at,
        }));
      
      return mappedStaff;
    }
    
    // If the response doesn't have the expected structure, return an empty array
    console.warn("API response does not have the expected structure:", response);
    return [];
  } catch (error) {
    console.error("Error fetching staff:", error);
    // Return empty array instead of throwing error
    return [];
  }
});

// Define the Teacher response type
interface TeacherResponse {
  id: string;
  name: string;
  first_name: string;
  last_name: string;
  specialization: string;
  role: string | null;
  qualifications: string | null;
  tag_id: string | null;
  staff_notes: string | null;
  created_at: string;
  updated_at: string;
}

/**
 * Handler for POST /api/staff
 * Creates a new staff member (potentially a teacher)
 */
export const POST = createPostHandler<TeacherResponse, StaffCreateRequest>(
  async (_request: NextRequest, body: StaffCreateRequest, token: string) => {
    // Validate required fields
    if (!body.person_id || body.person_id <= 0) {
      throw new Error('Missing required field: person_id must be a positive number');
    }
    
    // If creating a teacher, specialization is required
    if (body.is_teacher && (!body.specialization || body.specialization.trim() === '')) {
      throw new Error('Specialization is required for teachers');
    }
    
    try {
      // Create the staff member via the API
      const response = await apiPost<BackendStaffResponse>("/api/staff", token, body);
      
      // Map the response to match the Teacher interface from teacher-api.ts
      return {
        ...response,
        id: String(response.id),
        name: response.person ? `${response.person.first_name} ${response.person.last_name}` : "",
        first_name: response.person?.first_name ?? "",
        last_name: response.person?.last_name ?? "",
        specialization: response.specialization ?? "",
        role: response.role ?? null,
        qualifications: response.qualifications ?? null,
        tag_id: response.person?.tag_id ?? null,
        staff_notes: response.staff_notes ?? null,
      };
    } catch (error) {
      // Check for permission errors (403 Forbidden)
      if (error instanceof Error && error.message.includes("403")) {
        console.error("Permission denied when creating staff:", error);
        throw new Error("Permission denied: You need the 'users:create' permission to create staff members.");
      }
      
      // Check for validation errors 
      if (error instanceof Error && error.message.includes("400")) {
        const errorMessage = error.message;
        console.error("Validation error when creating staff:", errorMessage);
        
        // Extract specific error message if possible
        if (errorMessage.includes("person not found")) {
          throw new Error("Person not found with the specified ID");
        }
        if (errorMessage.includes("specialization is required")) {
          throw new Error("Specialization is required for teachers");
        }
      }
      
      // Re-throw other errors
      throw error;
    }
  }
);