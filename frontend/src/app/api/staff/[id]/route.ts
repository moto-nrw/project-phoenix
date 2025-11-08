// app/api/staff/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers";
import {
  createGetHandler,
  createPutHandler,
  createDeleteHandler,
} from "~/lib/route-wrapper";

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
    account_id?: number;
    created_at: string;
    updated_at: string;
  };
  created_at: string;
  updated_at: string;
}

/**
 * Type definition for staff update request
 */
interface StaffUpdateRequest {
  person_id?: number;
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
  data: BackendStaffResponse;
}

/**
 * Handler for GET /api/staff/[id]
 * Returns a single staff member by ID
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    if (!id) {
      throw new Error("Staff ID is required");
    }

    try {
      // Fetch staff member from backend API
      const response = await apiGet<ApiStaffResponse>(
        `/api/staff/${id}`,
        token,
      );

      // Handle null or undefined response
      if (!response?.data) {
        console.warn("API returned null response for staff member");
        throw new Error("Staff member not found");
      }

      const staff = response.data;

      // Map the response data to match the Teacher interface from teacher-api.ts
      return {
        id: String(staff.id), // This should be the staff ID since that's what we use for the API
        name: staff.person
          ? `${staff.person.first_name} ${staff.person.last_name}`
          : "",
        first_name: staff.person?.first_name ?? "",
        last_name: staff.person?.last_name ?? "",
        email: staff.person?.email ?? undefined, // Include email from person object
        specialization: staff.specialization ?? "",
        role: staff.role ?? null,
        qualifications: staff.qualifications ?? null,
        tag_id: staff.person?.tag_id ?? null,
        staff_notes: staff.staff_notes ?? null,
        created_at: staff.created_at,
        updated_at: staff.updated_at,
        // Include person_id for updates
        person_id: staff.person_id,
        // Include account_id from person object
        account_id: staff.person?.account_id,
        // Include both IDs for debugging
        staff_id: String(staff.id),
        teacher_id: staff.teacher_id ? String(staff.teacher_id) : undefined,
        // Include person object if available
        person: staff.person,
      };
    } catch (error) {
      console.error("Error fetching staff member:", error);
      throw error;
    }
  },
);

// Define the Teacher response type
interface TeacherResponse {
  id: string;
  name: string;
  first_name: string;
  last_name: string;
  email?: string;
  specialization: string;
  role: string | null;
  qualifications: string | null;
  tag_id: string | null;
  staff_notes: string | null;
  created_at: string;
  updated_at: string;
  person_id?: number;
  account_id?: number;
  staff_id?: string;
  teacher_id?: string;
  person?: {
    id: number;
    first_name: string;
    last_name: string;
    email?: string;
    tag_id?: string;
    account_id?: number;
    created_at: string;
    updated_at: string;
  };
}

/**
 * Handler for PUT /api/staff/[id]
 * Updates an existing staff member
 */
export const PUT = createPutHandler<TeacherResponse, StaffUpdateRequest>(
  async (
    _request: NextRequest,
    body: StaffUpdateRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    if (!id) {
      throw new Error("Staff ID is required");
    }

    // If updating a teacher, specialization is required
    if (
      body.is_teacher &&
      (!body.specialization || body.specialization.trim() === "")
    ) {
      throw new Error("Specialization is required for teachers");
    }

    try {
      // Update the staff member via the API
      const response = await apiPut<BackendStaffResponse>(
        `/api/staff/${id}`,
        token,
        body,
      );

      // Map the response to match the Teacher interface from teacher-api.ts
      return {
        id: String(response.id),
        name: response.person
          ? `${response.person.first_name} ${response.person.last_name}`
          : "",
        first_name: response.person?.first_name ?? "",
        last_name: response.person?.last_name ?? "",
        email: response.person?.email ?? undefined, // Include email from person object
        specialization: response.specialization ?? "",
        role: response.role ?? null,
        qualifications: response.qualifications ?? null,
        tag_id: response.person?.tag_id ?? null,
        staff_notes: response.staff_notes ?? null,
        created_at: response.created_at,
        updated_at: response.updated_at,
        // Include account_id from person object
        account_id: response.person?.account_id,
        // Include person_id for updates
        person_id: response.person_id,
        // Include person object if available
        person: response.person,
      };
    } catch (error) {
      // Check for permission errors (403 Forbidden)
      if (error instanceof Error && error.message.includes("403")) {
        console.error("Permission denied when updating staff:", error);
        throw new Error(
          "Permission denied: You need the 'users:update' permission to update staff members.",
        );
      }

      // Check for validation errors
      if (error instanceof Error && error.message.includes("400")) {
        const errorMessage = error.message;
        console.error("Validation error when updating staff:", errorMessage);

        // Extract specific error message if possible
        if (errorMessage.includes("staff member not found")) {
          throw new Error("Staff member not found");
        }
        if (errorMessage.includes("specialization is required")) {
          throw new Error("Specialization is required for teachers");
        }
      }

      // Re-throw other errors
      throw error;
    }
  },
);

/**
 * Handler for DELETE /api/staff/[id]
 * Deletes a staff member
 */
export const DELETE = createDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    if (!id) {
      throw new Error("Staff ID is required");
    }

    try {
      // Delete the staff member via the API
      await apiDelete(`/api/staff/${id}`, token);

      // Return null to indicate success with no content
      return null;
    } catch (error) {
      // Check for permission errors (403 Forbidden)
      if (error instanceof Error && error.message.includes("403")) {
        console.error("Permission denied when deleting staff:", error);
        throw new Error(
          "Permission denied: You need the 'users:delete' permission to delete staff members.",
        );
      }

      // Check for not found errors
      if (error instanceof Error && error.message.includes("404")) {
        throw new Error("Staff member not found");
      }

      // Re-throw other errors
      throw error;
    }
  },
);
