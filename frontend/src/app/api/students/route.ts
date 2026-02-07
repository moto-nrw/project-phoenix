// app/api/students/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost, apiPut } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentsRoute" });
import type { Student } from "~/lib/student-helpers";
import { mapStudentResponse } from "~/lib/student-helpers";
import {
  shouldCreatePrivacyConsent,
  updatePrivacyConsent,
  fetchPrivacyConsent,
} from "~/lib/student-privacy-helpers";
import {
  validateStudentFields,
  parseGuardianContact,
  buildBackendStudentRequest,
  handlePrivacyConsentCreation,
  buildStudentResponse,
  handleStudentCreationError,
} from "~/lib/student-request-helpers";

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
export const GET = createGetHandler(
  async (
    request: NextRequest,
    token: string,
  ): Promise<PaginatedStudentsResponse> => {
    // Build URL with any query parameters
    const queryParams = new URLSearchParams();
    request.nextUrl.searchParams.forEach((value, key) => {
      queryParams.append(key, value);
    });

    // Override page_size to load all students at once
    queryParams.set("page_size", "1000");

    const endpoint = `/api/students${queryParams.toString() ? "?" + queryParams.toString() : ""}`;

    try {
      // Fetch students from backend API
      const response = await apiGet<ApiStudentsResponse>(endpoint, token);

      // Handle null or undefined response
      if (!response) {
        logger.warn("API returned null response for students");
        return {
          data: [],
          pagination: {
            current_page: 1,
            page_size: 50,
            total_pages: 0,
            total_records: 0,
          },
        };
      }

      // Check for the paginated response structure from backend
      if ("data" in response && Array.isArray(response.data)) {
        // Map the backend response format to the frontend format using the consistent mapping function
        const mappedStudents = response.data.map(
          (student: StudentResponseFromBackend) => {
            const mapped = mapStudentResponse(student);
            return mapped;
          },
        );

        return {
          data: mappedStudents,
          pagination: response.pagination,
        };
      }

      // If the response doesn't have the expected structure, return empty paginated response
      logger.warn("students API response has unexpected structure");
      return {
        data: [],
        pagination: {
          current_page: 1,
          page_size: 50,
          total_pages: 0,
          total_records: 0,
        },
      };
    } catch (error) {
      logger.error("students fetch failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      throw error; // Re-throw to let the error handler deal with it
    }
  },
);

/**
 * Handler for POST /api/students
 * Creates a new student with associated person record
 */
export const POST = createPostHandler<
  Student,
  Omit<Student, "id"> & {
    guardian_email?: string;
    guardian_phone?: string;
    privacy_consent_accepted?: boolean;
    data_retention_days?: number;
  }
>(
  async (
    _request: NextRequest,
    body: Omit<Student, "id"> & {
      guardian_email?: string;
      guardian_phone?: string;
      privacy_consent_accepted?: boolean;
      data_retention_days?: number;
    },
    token: string,
  ) => {
    // Extract privacy consent fields
    const { privacy_consent_accepted, data_retention_days } = body;

    // Validate required fields
    const validated = validateStudentFields(body);

    // Parse guardian contact information
    const guardianContact = parseGuardianContact(
      body.guardian_email,
      body.guardian_phone,
      body.contact_lg,
    );

    // Build backend request
    const backendRequest = buildBackendStudentRequest(
      validated,
      body,
      guardianContact,
    );

    try {
      // Create the student via the simplified API endpoint
      const rawResponse = await apiPost<{
        status: string;
        data: StudentResponseFromBackend;
        message: string;
      }>("/api/students", token, backendRequest as StudentResponseFromBackend);

      const response = rawResponse.data;

      // Handle privacy consent (non-critical, errors are logged but not thrown)
      if (response?.id) {
        await handlePrivacyConsentCreation(
          response.id,
          privacy_consent_accepted,
          data_retention_days,
          apiPut,
          token,
          shouldCreatePrivacyConsent,
          updatePrivacyConsent,
        );
      }

      // Map response and fetch privacy consent data
      const mappedStudent = mapStudentResponse(response);
      return await buildStudentResponse(
        mappedStudent,
        response?.id,
        apiGet,
        token,
        fetchPrivacyConsent,
      );
    } catch (error) {
      handleStudentCreationError(error);
    }
  },
);
