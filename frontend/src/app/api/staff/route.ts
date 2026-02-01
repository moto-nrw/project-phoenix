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
  was_present_today?: boolean;
  work_status?: string;
  absence_type?: string;
}

/**
 * Type definition for staff creation request
 */
interface StaffCreateRequest {
  person_id: number;
  staff_notes?: string | null;
  is_teacher?: boolean;
  specialization?: string | null;
  role?: string | null;
  qualifications?: string | null;
}

/**
 * Normalizes an optional string field for API submission.
 * - If original is undefined, returns undefined (field not provided)
 * - If trimmed value is empty, returns empty string (field explicitly cleared)
 * - Otherwise returns the trimmed value
 */
function normalizeOptionalString(
  original: string | null | undefined,
  trimmed: string | undefined,
): string | undefined {
  if (original === undefined) {
    return undefined;
  }
  return trimmed === "" ? "" : trimmed;
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
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    // Build URL with any query parameters
    const queryParams = new URLSearchParams();
    request.nextUrl.searchParams.forEach((value, key) => {
      queryParams.append(key, value);
    });

    const endpoint = `/api/staff${queryParams.toString() ? "?" + queryParams.toString() : ""}`;

    console.log("Requesting staff from backend:", endpoint);

    try {
      // Fetch staff from backend API
      const response = await apiGet<BackendStaffResponse[] | ApiStaffResponse>(
        endpoint,
        token,
      );

      // Handle null or undefined response
      if (!response) {
        console.warn("API returned null response for staff");
        return [];
      }

      // Debug output to check the response data
      console.log("API staff response:", JSON.stringify(response, null, 2));

      // Check if the response is already an array (common pattern)
      if (Array.isArray(response)) {
        // Direct array response - map all staff (not just teachers)
        const mappedStaff = response.map((staff: BackendStaffResponse) => ({
          id: String(staff.id), // Always use staff.id as unique identifier (NEVER teacher_id to avoid duplicates)
          name: staff.person
            ? `${staff.person.first_name} ${staff.person.last_name}`
            : "",
          firstName: staff.person?.first_name ?? "",
          lastName: staff.person?.last_name ?? "",
          specialization: staff.specialization ?? null,
          role: staff.role ?? null,
          qualifications: staff.qualifications ?? null,
          tag_id: staff.person?.tag_id ?? null,
          staff_notes: staff.staff_notes ?? null,
          created_at: staff.created_at,
          updated_at: staff.updated_at,
          // Include both IDs for reference
          staff_id: String(staff.id),
          teacher_id: staff.teacher_id ? String(staff.teacher_id) : undefined,
          person_id: staff.person_id,
          was_present_today: staff.was_present_today,
          work_status: staff.work_status,
          absence_type: staff.absence_type,
        }));

        return mappedStaff;
      }

      // Check for nested data structure
      if ("data" in response && Array.isArray(response.data)) {
        // Map the response data to match the Teacher interface from teacher-api.ts
        const mappedStaff = response.data.map(
          (staff: BackendStaffResponse) => ({
            id: String(staff.id), // Always use staff.id as unique identifier (NEVER teacher_id to avoid duplicates)
            name: staff.person
              ? `${staff.person.first_name} ${staff.person.last_name}`
              : "",
            firstName: staff.person?.first_name ?? "",
            lastName: staff.person?.last_name ?? "",
            specialization: staff.specialization ?? null,
            role: staff.role ?? null,
            qualifications: staff.qualifications ?? null,
            tag_id: staff.person?.tag_id ?? null,
            staff_notes: staff.staff_notes ?? null,
            created_at: staff.created_at,
            updated_at: staff.updated_at,
            // Include both IDs for reference
            staff_id: String(staff.id),
            teacher_id: staff.teacher_id ? String(staff.teacher_id) : undefined,
            person_id: staff.person_id,
            was_present_today: staff.was_present_today,
            work_status: staff.work_status,
            absence_type: staff.absence_type,
          }),
        );

        return mappedStaff;
      }

      // If the response doesn't have the expected structure, return an empty array
      console.warn(
        "API response does not have the expected structure:",
        response,
      );
      return [];
    } catch (error) {
      console.error("Error fetching staff:", error);
      // Return empty array instead of throwing error
      return [];
    }
  },
);

// Define the Teacher response type
interface TeacherResponse {
  id: string;
  name: string;
  firstName: string;
  lastName: string;
  specialization?: string | null;
  role: string | null;
  qualifications: string | null;
  tag_id: string | null;
  staff_notes: string | null;
  created_at: string;
  updated_at: string;
  staff_id?: string;
  teacher_id?: string;
}

/**
 * Handler for POST /api/staff
 * Creates a new staff member (potentially a teacher)
 */
export const POST = createPostHandler<TeacherResponse, StaffCreateRequest>(
  async (_request: NextRequest, body: StaffCreateRequest, token: string) => {
    // Validate required fields
    if (!body.person_id || body.person_id <= 0) {
      throw new Error(
        "Missing required field: person_id must be a positive number",
      );
    }

    try {
      const trimmedNotes = body.staff_notes?.trim();
      const trimmedSpecialization = body.specialization?.trim();
      const trimmedRole = body.role?.trim();
      const trimmedQualifications = body.qualifications?.trim();

      const normalizedBody: StaffCreateRequest = {
        ...body,
        staff_notes: normalizeOptionalString(body.staff_notes, trimmedNotes),
        specialization: normalizeOptionalString(
          body.specialization,
          trimmedSpecialization,
        ),
        role: normalizeOptionalString(body.role, trimmedRole),
        qualifications: normalizeOptionalString(
          body.qualifications,
          trimmedQualifications,
        ),
      };

      // Create the staff member via the API
      const response = await apiPost<BackendStaffResponse>(
        "/api/staff",
        token,
        normalizedBody,
      );

      // Map the response to match the Teacher interface from teacher-api.ts
      return {
        ...response,
        id: String(response.id), // Always use staff.id as unique identifier (NEVER teacher_id to avoid duplicates)
        name: response.person
          ? `${response.person.first_name} ${response.person.last_name}`
          : "",
        firstName: response.person?.first_name ?? "",
        lastName: response.person?.last_name ?? "",
        specialization: response.specialization ?? null,
        role: response.role ?? null,
        qualifications: response.qualifications ?? null,
        tag_id: response.person?.tag_id ?? null,
        staff_notes: response.staff_notes ?? null,
        staff_id: String(response.id),
        teacher_id: response.teacher_id
          ? String(response.teacher_id)
          : undefined,
        person_id: response.person_id,
      };
    } catch (error) {
      // Check for permission errors (403 Forbidden)
      if (error instanceof Error && error.message.includes("403")) {
        console.error("Permission denied when creating staff:", error);
        throw new Error(
          "Permission denied: You need the 'users:create' permission to create staff members.",
        );
      }

      // Check for validation errors
      if (error instanceof Error && error.message.includes("400")) {
        const errorMessage = error.message;
        console.error("Validation error when creating staff:", errorMessage);

        // Extract specific error message if possible
        if (errorMessage.includes("person not found")) {
          throw new Error("Person not found with the specified ID");
        }
        // No additional specialization validation anymore
      }

      // Re-throw other errors
      throw error;
    }
  },
);
