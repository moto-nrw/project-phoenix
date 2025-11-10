// app/api/students/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost, apiPut, apiDelete } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";
import type { Student } from "~/lib/student-helpers";
import { mapStudentResponse, prepareStudentForBackend } from "~/lib/student-helpers";
import { LOCATION_STATUSES } from "~/lib/location-helper";

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
  current_location?: string | null;
  bus: boolean;
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
 * Type definition for paginated response
 */
interface PaginatedStudentsResponse {
  data: Student[];
  pagination: {
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  };
}

/**
 * Handler for GET /api/students
 * Returns a paginated list of students, optionally filtered by query parameters
 */
export const GET = createGetHandler(async (request: NextRequest, token: string): Promise<PaginatedStudentsResponse> => {
  // Build URL with any query parameters
  const queryParams = new URLSearchParams();
  request.nextUrl.searchParams.forEach((value, key) => {
    queryParams.append(key, value);
  });
  
  // Override page_size to load all students at once
  queryParams.set('page_size', '1000');
  
  const endpoint = `/api/students${queryParams.toString() ? '?' + queryParams.toString() : ''}`;
  
  
  try {
    // Fetch students from backend API
    const response = await apiGet<ApiStudentsResponse>(endpoint, token);
    
    // Handle null or undefined response
    if (!response) {
      console.warn("API returned null response for students");
      return {
        data: [],
        pagination: {
          current_page: 1,
          page_size: 50,
          total_pages: 0,
          total_records: 0
        }
      };
    }
    
    
    // Check for the paginated response structure from backend
    if ('data' in response && Array.isArray(response.data)) {
      // Map the backend response format to the frontend format using the consistent mapping function
      const mappedStudents = response.data.map((student: StudentResponseFromBackend) => {
        const mapped = mapStudentResponse(student);
        return mapped;
      });
      
      return {
        data: mappedStudents,
        pagination: response.pagination
      };
    }
    
    // If the response doesn't have the expected structure, return empty paginated response
    console.warn("API response does not have the expected structure:", response);
    return {
      data: [],
      pagination: {
        current_page: 1,
        page_size: 50,
        total_pages: 0,
        total_records: 0
      }
    };
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
// Define type for backend request structure
interface BackendStudentRequest {
  first_name: string;
  last_name: string;
  school_class: string;
  // Legacy guardian fields (optional - use guardian system instead)
  guardian_name?: string;
  guardian_contact?: string;
  guardian_email?: string;
  guardian_phone?: string;
  // Other optional fields
  current_location?: string;
  notes?: string;
  tag_id?: string;
  group_id?: number;
  bus?: boolean;
  extra_info?: string;
}

export const POST = createPostHandler<Student, Omit<Student, "id"> & { guardian_email?: string; guardian_phone?: string; privacy_consent_accepted?: boolean; data_retention_days?: number; }>(
  async (_request: NextRequest, body: Omit<Student, "id"> & { guardian_email?: string; guardian_phone?: string; privacy_consent_accepted?: boolean; data_retention_days?: number; }, token: string) => {
    // Extract privacy consent fields
    const { privacy_consent_accepted, data_retention_days, ...studentData } = body;
    
    // Transform frontend format to backend format
    const backendData = prepareStudentForBackend(studentData);
    
    // Extract guardian email/phone from contact_lg if not provided separately
    let guardianEmail = body.guardian_email;
    let guardianPhone = body.guardian_phone;
    
    if (!guardianEmail && !guardianPhone && body.contact_lg) {
      // Parse guardian contact - check if it's an email or phone
      if (body.contact_lg.includes('@')) {
        guardianEmail = body.contact_lg;
      } else {
        guardianPhone = body.contact_lg;
      }
    }
    
    // Validate required fields using frontend field names
    const firstName = body.first_name?.trim();
    const lastName = body.second_name?.trim();
    const schoolClass = body.school_class?.trim();
    const guardianName = body.name_lg?.trim();
    const guardianContact = body.contact_lg?.trim();

    if (!firstName) {
      throw new Error('First name is required');
    }

    if (!lastName) {
      throw new Error('Last name is required');
    }

    if (!schoolClass) {
      throw new Error('School class is required');
    }

    // Guardian fields are now optional (legacy fields - use guardian system instead)
    // No validation required for guardian fields
    
    // Create a properly typed request object using the transformed data
    const backendRequest: BackendStudentRequest = {
      first_name: firstName,
      last_name: lastName,
      school_class: schoolClass,
      current_location: backendData.current_location ?? LOCATION_STATUSES.UNKNOWN,
      notes: undefined, // Not in frontend model
      tag_id: backendData.tag_id,
      group_id: backendData.group_id,
      bus: backendData.bus,
      extra_info: backendData.extra_info
    };

    // Only include legacy guardian fields if provided
    if (guardianName) {
      backendRequest.guardian_name = guardianName;
    }
    if (guardianContact) {
      backendRequest.guardian_contact = guardianContact;
    }
    if (guardianEmail || backendData.guardian_email) {
      backendRequest.guardian_email = guardianEmail ?? backendData.guardian_email;
    }
    if (guardianPhone || backendData.guardian_phone) {
      backendRequest.guardian_phone = guardianPhone ?? backendData.guardian_phone;
    }
    
    try {
      // Create the student via the simplified API endpoint
      const rawResponse = await apiPost<{ status: string; data: StudentResponseFromBackend; message: string }>("/api/students", token, backendRequest as StudentResponseFromBackend);
      
      // Extract the student data from the response
      const response = rawResponse.data;
      
      // Handle privacy consent if provided
      if ((privacy_consent_accepted !== undefined || data_retention_days !== undefined) && response?.id) {
        try {
          await apiPut(`/api/students/${response.id}/privacy-consent`, token, {
            policy_version: "1.0",
            accepted: privacy_consent_accepted ?? false,
            data_retention_days: data_retention_days ?? 30,
          });
        } catch (consentError) {
          console.error("Error creating privacy consent:", consentError);
          // GDPR: Ensure atomicity — roll back student creation if consent fails
          try {
            await apiDelete(`/api/students/${response.id}`, token);
          } catch (rollbackError) {
            console.error("Failed to rollback student after consent error:", rollbackError);
          }
          throw new Error("Datenschutzeinwilligung konnte nicht erstellt werden. Vorgang abgebrochen.");
        }
      }
      
      // Map the backend response to frontend format using the consistent mapping function
      return mapStudentResponse(response);
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
