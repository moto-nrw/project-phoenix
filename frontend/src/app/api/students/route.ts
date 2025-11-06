// app/api/students/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost, apiPut, apiDelete } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";
import type { Student } from "~/lib/student-helpers";
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
export const POST = createPostHandler<Student, Omit<Student, "id"> & { guardian_email?: string; guardian_phone?: string; privacy_consent_accepted?: boolean; data_retention_days?: number; guardians?: Array<{
  id: number;
  first_name: string;
  last_name: string;
  email: string;
  phone: string;
  relationship_type: string;
  is_primary: boolean;
  is_emergency_contact: boolean;
  can_pickup: boolean;
}>; }>(
  async (_request: NextRequest, body: Omit<Student, "id"> & { guardian_email?: string; guardian_phone?: string; privacy_consent_accepted?: boolean; data_retention_days?: number; guardians?: Array<{
    id: number;
    first_name: string;
    last_name: string;
    email: string;
    phone: string;
    relationship_type: string;
    is_primary: boolean;
    is_emergency_contact: boolean;
    can_pickup: boolean;
  }>; }, token: string) => {
    // Extract privacy consent fields and guardians array
    const { privacy_consent_accepted, data_retention_days, guardians, ...studentData } = body;

    // Validate required fields using frontend field names
    const firstName = body.first_name?.trim();
    const lastName = body.second_name?.trim();
    const schoolClass = body.school_class?.trim();

    if (!firstName) {
      throw new Error('First name is required');
    }

    if (!lastName) {
      throw new Error('Last name is required');
    }

    if (!schoolClass) {
      throw new Error('School class is required');
    }

    // Guardian validation - use guardians array or fall back to legacy fields
    const guardiansArray = guardians && guardians.length > 0 ? guardians : [];

    // If no guardians array, fall back to legacy name_lg field
    if (guardiansArray.length === 0) {
      const guardianName = body.name_lg?.trim();
      if (!guardianName) {
        throw new Error('At least one guardian is required');
      }

      // Split guardian full name into first and last name
      const nameParts = guardianName.split(' ');
      const guardianFirstName = nameParts[0] ?? '';
      const guardianLastName = nameParts.length > 1 ? nameParts.slice(1).join(' ') : guardianFirstName;

      // Extract guardian contact info - email and phone are now optional
      // eslint-disable-next-line @typescript-eslint/prefer-nullish-coalescing
      const guardianEmail = body.guardian_email?.trim() || undefined;
      // eslint-disable-next-line @typescript-eslint/prefer-nullish-coalescing
      const guardianPhone = (body.guardian_phone?.trim() ?? body.contact_lg?.trim()) || undefined;

      guardiansArray.push({
        id: 0,
        first_name: guardianFirstName,
        last_name: guardianLastName,
        email: guardianEmail ?? '',
        phone: guardianPhone ?? '',
        relationship_type: 'parent',
        is_primary: true,
        is_emergency_contact: true,
        can_pickup: true
      });
    }

    // Filter out guardians with empty names
    const validGuardians = guardiansArray.filter(g => g.first_name.trim() && g.last_name.trim());

    if (validGuardians.length === 0) {
      throw new Error('At least one guardian with name is required');
    }

    // First guardian (primary) will be created with the student
    const primaryGuardian = validGuardians[0]!; // Non-null assertion safe because we validated length > 0
    const guardianRequest = {
      first_name: primaryGuardian.first_name.trim(),
      last_name: primaryGuardian.last_name.trim(),
      email: primaryGuardian.email.trim() || undefined,
      phone: primaryGuardian.phone.trim() || undefined,
      relationship_type: primaryGuardian.relationship_type || 'parent'
    };

    // Create a properly typed request object for the NEW backend API
    const backendRequest = {
      first_name: firstName,
      last_name: lastName,
      school_class: schoolClass,
      guardian: guardianRequest, // New guardian object format (only first guardian)
      // eslint-disable-next-line @typescript-eslint/prefer-nullish-coalescing
      tag_id: body.studentId?.trim() || undefined,
      group_id: body.group_id ? parseInt(body.group_id, 10) : undefined,
      bus: body.bus ?? false,
      extra_info: studentData.extra_info
    };

    try {
      // Create the student with the first (primary) guardian
      const rawResponse = await apiPost<{ status: string; data: StudentResponseFromBackend; message: string }>("/api/students", token, backendRequest);

      // Extract the student data from the response
      const response = rawResponse.data;

      if (!response?.id) {
        throw new Error('Failed to create student - no ID returned');
      }

      // Create additional guardians (if any) via the guardian endpoint
      if (validGuardians.length > 1) {
        for (let i = 1; i < validGuardians.length; i++) {
          const additionalGuardian = validGuardians[i]!; // Non-null assertion safe because loop is within bounds
          try {
            await apiPost(
              `/api/students/${response.id}/guardians`,
              token,
              {
                first_name: additionalGuardian.first_name.trim(),
                last_name: additionalGuardian.last_name.trim(),
                email: additionalGuardian.email.trim() || undefined,
                phone: additionalGuardian.phone.trim() || undefined,
                relationship_type: additionalGuardian.relationship_type || 'parent',
                is_primary: false, // Only first guardian is primary
                is_emergency_contact: additionalGuardian.is_emergency_contact,
                can_pickup: additionalGuardian.can_pickup
              }
            );
          } catch (guardianError) {
            console.error(`Error creating additional guardian ${i}:`, guardianError);
            // Continue with other guardians even if one fails
            // The student is already created, so we don't want to roll back everything
          }
        }
      }

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
