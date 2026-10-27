// app/api/activities/[id]/students/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost, apiPut, bffError } from "~/lib/api-helpers.server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ActivityStudentsRoute" });
import {
  createGetHandler,
  createPostHandler,
  createPutHandler,
} from "~/lib/route-wrapper.server";
import {
  mapStudentEnrollmentsResponse,
  type BackendStudentEnrollment,
} from "~/lib/activity-helpers";

interface StudentListPage {
  data?: Array<{
    id: number;
    first_name: string;
    last_name: string;
    school_class?: string;
  }>;
  pagination?: {
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  };
}

// Reject oversized/inconsistent responses, never truncate them into a success.
const MAX_STUDENT_PAGES = 1_000;

function failStudentPagination(reason: string): never {
  logger.error("invalid_student_pagination", { reason });
  throw bffError(502, "Invalid student list pagination", {
    code: "general.unavailable",
  });
}

async function fetchAllStudents(params: URLSearchParams, token: string) {
  const firstResponse = await apiGet<StudentListPage>(
    `/api/students?${params.toString()}`,
    token,
  );
  const pagination = firstResponse.pagination;
  if (!pagination) failStudentPagination("missing pagination");
  // Freeze the traversal plan; later responses cannot extend the work.
  const {
    total_pages: totalPages,
    total_records: totalRecords,
    page_size: pageSize,
  } = pagination;
  if (
    !Number.isSafeInteger(totalPages) ||
    totalPages < 0 ||
    totalPages > MAX_STUDENT_PAGES ||
    !Number.isSafeInteger(totalRecords) ||
    totalRecords < 0 ||
    !Number.isSafeInteger(pageSize) ||
    pageSize < 1 ||
    totalPages !== Math.ceil(totalRecords / pageSize)
  ) {
    failStudentPagination("invalid pagination bounds");
  }

  const students: NonNullable<StudentListPage["data"]> = [];
  const seenIds = new Set<number>();
  for (let page = 1; page <= Math.max(1, totalPages); page++) {
    let response = firstResponse;
    if (page > 1) {
      params.set("page", String(page));
      response = await apiGet<StudentListPage>(
        `/api/students?${params.toString()}`,
        token,
      );
    }
    if (
      response.pagination?.current_page !== page ||
      response.pagination.page_size !== pageSize ||
      response.pagination.total_pages !== totalPages ||
      response.pagination.total_records !== totalRecords
    ) {
      failStudentPagination("page metadata changed");
    }
    const data = response.data ?? [];
    const expectedCount = Math.min(
      pageSize,
      totalRecords - (page - 1) * pageSize,
    );
    if (!Array.isArray(data) || data.length !== expectedCount) {
      failStudentPagination("unexpected page length");
    }
    for (const student of data) {
      if (
        !student ||
        !Number.isSafeInteger(student.id) ||
        student.id < 1 ||
        seenIds.has(student.id)
      ) {
        failStudentPagination("invalid or repeated student ID");
      }
      seenIds.add(student.id);
      students.push(student);
    }
  }
  if (students.length !== totalRecords) {
    failStudentPagination("student count differs from total_records");
  }
  return students;
}

/**
 * Handler for GET /api/activities/[id]/students
 * Returns a list of students enrolled in a specific activity
 */
export const GET = createGetHandler(
  async (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    // Check for "available" query parameter - return available students instead of enrolled ones
    const available = request.nextUrl.searchParams.get("available") === "true";

    if (available) {
      // Get query parameters for filtering
      const search = request.nextUrl.searchParams.get("search") ?? undefined;
      const groupId = request.nextUrl.searchParams.get("group_id") ?? undefined;

      // Get available students by fetching all students and filtering out enrolled ones
      try {
        const allStudentsParams = new URLSearchParams();
        if (search) allStudentsParams.append("search", search);
        if (groupId) allStudentsParams.append("group_id", groupId);

        // The roster and paginated candidate list are independent reads.
        const [enrolledResponse, allStudents] = await Promise.all([
          apiGet<{ data: BackendStudentEnrollment[] }>(
            `/api/activities/${id}/students`,
            token,
          ),
          fetchAllStudents(allStudentsParams, token),
        ]);
        const enrolledStudentIds = new Set(
          (enrolledResponse.data ?? []).map((e) => e.id),
        );

        // Filter out enrolled students and format for frontend
        const availableStudents = allStudents
          .filter((student) => !enrolledStudentIds.has(student.id))
          .map((student) => ({
            id: String(student.id),
            name: `${student.first_name} ${student.last_name}`.trim(),
            school_class: student.school_class ?? "",
          }));

        return availableStudents;
      } catch (error) {
        logger.error("failed to fetch available students", {
          error: error instanceof Error ? error.message : String(error),
        });
        throw error;
      }
    }

    // Otherwise return enrolled students - call backend directly
    try {
      const endpoint = `/api/activities/${id}/students`;
      const response = await apiGet<{ data: BackendStudentEnrollment[] }>(
        endpoint,
        token,
      );
      return mapStudentEnrollmentsResponse(response.data ?? []);
    } catch (error) {
      logger.error("failed to fetch enrolled students", {
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },
);

/**
 * Handler for POST /api/activities/[id]/students
 * Supports two modes:
 * 1. Regular: Enrolls a single student - expects { student_id: string }
 * 2. Batch: Updates multiple students at once - expects { student_ids: string[] }
 */
export const POST = createPostHandler(
  async (
    request: NextRequest,
    body: { student_ids?: string[]; student_id?: string },
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    // Check if this is a batch update operation
    if (body.student_ids && Array.isArray(body.student_ids)) {
      await apiPut(`/api/activities/${id}/students`, token, {
        student_ids: body.student_ids.map((studentId) =>
          Number.parseInt(studentId, 10),
        ),
      });
      const response = await apiGet<{ data: BackendStudentEnrollment[] }>(
        `/api/activities/${id}/students`,
        token,
      );
      return mapStudentEnrollmentsResponse(response.data ?? []);
    }
    // Regular single student enrollment
    else if (body.student_id) {
      const studentId = String(body.student_id);
      await apiPost(`/api/activities/${id}/enroll/${studentId}`, token, {});
      const response = await apiGet<{ data: BackendStudentEnrollment[] }>(
        `/api/activities/${id}/students`,
        token,
      );
      return mapStudentEnrollmentsResponse(response.data ?? []);
    } else {
      throw new Error(
        "Invalid request: must provide student_id or student_ids",
      );
    }
  },
);

/**
 * Handler for PUT /api/activities/[id]/students
 * Updates the complete list of enrolled students (replaces existing enrollments)
 */
export const PUT = createPutHandler(
  async (
    _request: NextRequest,
    body: { student_ids: string[] },
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    if (!body.student_ids || !Array.isArray(body.student_ids)) {
      throw new Error("student_ids array is required");
    }

    // Convert string IDs to numbers for backend
    const studentIds = body.student_ids.map((id) => Number.parseInt(id, 10));

    // Call backend to update enrollments
    const endpoint = `/api/activities/${id}/students`;
    await apiPut(endpoint, token, { student_ids: studentIds });

    // Return updated enrolled students - call backend directly with token
    const response = await apiGet<{ data: BackendStudentEnrollment[] }>(
      endpoint,
      token,
    );
    return mapStudentEnrollmentsResponse(response.data ?? []);
  },
);
