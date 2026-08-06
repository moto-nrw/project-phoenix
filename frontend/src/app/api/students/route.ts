// app/api/students/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost, apiPut } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentsRoute" });
import type { BackendSlimStudent, Student } from "~/lib/student-helpers";
import {
  mapSlimStudentResponse,
  mapStudentResponse,
} from "~/lib/student-helpers";
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
import type { StudentGuardianPayload } from "~/lib/guardian-helpers";
import type { ArrivalScheduleFormEntry } from "~/lib/arrival-schedule-helpers";
import type { BackendPickupScheduleRequest } from "~/lib/pickup-schedule-helpers";

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
  // Hex of the student's current room when set (Issue #1324). The proxy
  // forwards it untouched; mapStudentResponse threads it into the Student
  // type that powers LocationBadge.
  current_room_color?: string | null;
  bus: boolean;
  guardian_name: string;
  guardian_contact: string;
  guardian_email?: string;
  guardian_phone?: string;
  group_id?: number;
  day_planning_status?: "comes_today" | "not_coming_today";
  day_planning_reason?: string;
  day_planning_label?: string;
  // Parent's note for a still-pending "entschuldigt" request covering today;
  // forwarded untouched and threaded into Student by mapStudentResponse.
  pending_excused_note?: string;
  created_at: string;
  updated_at: string;
}

/**
 * Type definition for API response format from backend
 */
interface ApiStudentsResponse {
  status: string;
  data: Array<StudentResponseFromBackend | BackendSlimStudent>;
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

    // Default page_size to 1000 so callers that don't paginate (Kindersuche,
    // database list views) still get every row in one trip, but honor an
    // explicit page_size from the client. The room-detail modal sends 200
    // and relies on a smaller payload to drive its own truncation/overflow
    // notice (#1374) — overwriting it here would render up to 1000 cards
    // and silently disable the cap the UX depends on. Cap any client-
    // supplied value at MAX_PAGE_SIZE so an authenticated caller can't
    // force an arbitrarily large query (e.g. ?page_size=1000000).
    const MAX_PAGE_SIZE = 1000;
    const DEFAULT_PAGE_SIZE = 1000;
    const rawPageSize = queryParams.get("page_size");
    const parsedPageSize = rawPageSize !== null ? Number(rawPageSize) : NaN;
    const pageSize =
      Number.isInteger(parsedPageSize) && parsedPageSize > 0
        ? Math.min(parsedPageSize, MAX_PAGE_SIZE)
        : DEFAULT_PAGE_SIZE;
    queryParams.set("page_size", String(pageSize));
    const isSlimView = queryParams.get("view") === "slim";

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
        const mappedStudents = response.data.map((student) =>
          isSlimView
            ? mapSlimStudentResponse(student as BackendSlimStudent)
            : mapStudentResponse(student as StudentResponseFromBackend),
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
      const errorMessage =
        error instanceof Error ? error.message : String(error);
      const logContext = {
        error: errorMessage,
        ...(errorMessage.includes("API error (429)") && {
          rate_limited: true,
        }),
      };
      if (logContext.rate_limited) {
        logger.warn("students fetch failed", logContext);
      } else {
        logger.error("students fetch failed", logContext);
      }
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
    guardians?: StudentGuardianPayload[];
    arrival_schedules?: ArrivalScheduleFormEntry[];
    pickup_schedules?: BackendPickupScheduleRequest[];
  }
>(
  async (
    _request: NextRequest,
    body: Omit<Student, "id"> & {
      guardian_email?: string;
      guardian_phone?: string;
      privacy_consent_accepted?: boolean;
      data_retention_days?: number;
      guardians?: StudentGuardianPayload[];
      arrival_schedules?: ArrivalScheduleFormEntry[];
      pickup_schedules?: BackendPickupScheduleRequest[];
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
