// app/api/students/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers";
import {
  createGetHandler,
  createPutHandler,
  createDeleteHandler,
} from "~/lib/route-wrapper";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentDetailRoute" });
import type { BackendStudent, Student } from "~/lib/student-helpers";
import {
  mapStudentResponse,
  prepareStudentForBackend,
} from "~/lib/student-helpers";
import {
  fetchPrivacyConsent,
  updatePrivacyConsent,
} from "~/lib/student-privacy-helpers";

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
  current_location?: string | null;
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
 * Returns a single student by ID with privacy consent data
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    if (!id) {
      throw new Error("Student ID is required");
    }

    try {
      // Fetch student from backend API
      // Using unknown type and will validate structure
      const response = await apiGet<unknown>(`/api/students/${id}`, token);

      // Type guard to check response structure
      if (!response || typeof response !== "object" || !("data" in response)) {
        logger.warn("API returned invalid response for student", {
          student_id: id,
        });
        throw new Error("Student not found");
      }

      // Extract student data from the backend response
      // Backend sends: { status: "success", data: { ...student, has_full_access, group_supervisors } }

      // Define type for backend student data including access control fields
      interface BackendStudentData {
        last_name?: string;
        name?: string;
        first_name?: string;
        has_full_access?: boolean;
        group_supervisors?: Array<{
          id: number;
          first_name: string;
          last_name: string;
          email?: string;
          role: string;
        }>;
        [key: string]: unknown;
      }

      const backendResponse = response as {
        data: BackendStudentData;
      };

      // Map the backend response to frontend format
      const studentData = backendResponse.data;

      // Extract access control fields from response data
      const hasFullAccess = studentData.has_full_access ?? false;
      const groupSupervisors = studentData.group_supervisors ?? [];

      // Check if we need to extract last_name from the name field
      if (!studentData.last_name && studentData.name) {
        // Split the name to extract first and last name
        const nameParts = studentData.name.split(" ");
        if (nameParts.length > 1) {
          // If first_name matches the first part, the rest is the last name
          if (studentData.first_name === nameParts[0]) {
            studentData.last_name = nameParts.slice(1).join(" ");
          }
        }
      }

      const mappedStudent = mapStudentResponse(
        studentData as unknown as BackendStudent,
      );

      // Fetch privacy consent data using helper
      const consentData = await fetchPrivacyConsent(id, apiGet, token);

      return {
        ...mappedStudent,
        ...consentData,
        has_full_access: hasFullAccess,
        group_supervisors: groupSupervisors,
      };
    } catch (error) {
      logger.error("student fetch failed", {
        student_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },
);

/**
 * Handler for PUT /api/students/[id]
 * Updates an existing student
 */
export const PUT = createPutHandler<
  Student,
  Partial<Student> & {
    privacy_consent_accepted?: boolean;
    data_retention_days?: number;
  }
>(
  async (
    _request: NextRequest,
    body: Partial<Student> & {
      privacy_consent_accepted?: boolean;
      data_retention_days?: number;
    },
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    if (!id) {
      throw new Error("Student ID is required");
    }

    try {
      // Extract privacy consent fields
      const { privacy_consent_accepted, data_retention_days, ...studentData } =
        body;

      // Transform frontend format to backend format
      const backendData = prepareStudentForBackend(studentData);

      // Call backend API to update student
      const response = await apiPut<ApiStudentResponse>(
        `/api/students/${id}`,
        token,
        backendData,
      );

      // Handle null or undefined response
      if (!response?.data) {
        throw new Error("Invalid response from backend");
      }

      // Handle privacy consent if provided — fail update if consent update fails
      if (
        privacy_consent_accepted !== undefined ||
        data_retention_days !== undefined
      ) {
        try {
          await updatePrivacyConsent(
            id,
            apiPut,
            token,
            privacy_consent_accepted,
            data_retention_days,
            "PUT Student",
          );
        } catch (consentError) {
          logger.error("privacy consent update failed", {
            student_id: id,
            error:
              consentError instanceof Error
                ? consentError.message
                : String(consentError),
          });
          throw new Error(
            "Datenschutzeinstellungen konnten nicht aktualisiert werden.",
          );
        }
      }

      // Map the response to frontend format
      const mappedStudent = mapStudentResponse(response.data as BackendStudent);

      // Fetch privacy consent data to include in the response
      const consentData = await fetchPrivacyConsent(id, apiGet, token);

      return {
        ...mappedStudent,
        ...consentData,
      };
    } catch (error) {
      logger.error("student update failed", {
        student_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },
);

/**
 * Handler for PATCH /api/students/[id]
 * Partially updates a student (same as PUT but for PATCH requests)
 */
export const PATCH = PUT;

/**
 * Handler for DELETE /api/students/[id]
 * Deletes a student
 */
export const DELETE = createDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    if (!id) {
      throw new Error("Student ID is required");
    }

    try {
      // Call backend API to delete student
      const response = await apiDelete<{ message: string }>(
        `/api/students/${id}`,
        token,
      );

      return {
        success: true,
        message: response?.message ?? "Student deleted successfully",
      };
    } catch (error) {
      logger.error("student delete failed", {
        student_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },
);
