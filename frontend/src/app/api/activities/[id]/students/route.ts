// app/api/activities/[id]/students/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut } from "~/lib/api-helpers";
import {
  createGetHandler,
  createPostHandler,
  createPutHandler,
} from "~/lib/route-wrapper";
import { updateGroupEnrollments, enrollStudent } from "~/lib/activity-api";
import type { BackendStudentEnrollment } from "~/lib/activity-helpers";
import { mapStudentEnrollmentsResponse } from "~/lib/activity-helpers";

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
        // First, get enrolled students in this activity
        const enrolledResponse = await apiGet<{
          data: BackendStudentEnrollment[];
        }>(`/api/activities/${id}/students`, token);
        const enrolledStudentIds = new Set(
          (enrolledResponse.data ?? []).map((e) => e.id),
        );

        // Then, get all students
        const allStudentsParams = new URLSearchParams();
        if (search) allStudentsParams.append("search", search);
        if (groupId) allStudentsParams.append("group_id", groupId);

        const allStudentsUrl = `/api/students?${allStudentsParams.toString()}`;
        const allStudentsResponse = await apiGet<{
          data: Array<{
            id: number;
            first_name: string;
            last_name: string;
            school_class?: string;
          }>;
        }>(allStudentsUrl, token);

        // Filter out enrolled students and format for frontend
        const availableStudents = (allStudentsResponse.data ?? [])
          .filter((student) => !enrolledStudentIds.has(student.id))
          .map((student) => ({
            id: String(student.id),
            name: `${student.first_name} ${student.last_name}`.trim(),
            school_class: student.school_class ?? "",
          }));

        return availableStudents;
      } catch (error) {
        console.error("Error fetching available students:", error);
        return []; // Return empty array on error
      }
    }

    // Otherwise return enrolled students - call backend directly
    try {
      const endpoint = `/api/activities/${id}/students`;
      const response = await apiGet<{ data: BackendStudentEnrollment[] }>(
        endpoint,
        token,
      );
      const enrollments = response.data ?? [];
      // Map the backend enrollment structure to frontend format
      return mapStudentEnrollmentsResponse(enrollments);
    } catch (error) {
      console.error("Error fetching enrolled students:", error);
      return []; // Return empty array on error
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
      // Use batch update function
      const success = await updateGroupEnrollments(id, {
        student_ids: body.student_ids,
      });

      if (success) {
        // Return updated enrolled students - call backend directly with token
        const endpoint = `/api/activities/${id}/students`;
        const response = await apiGet<{ data: BackendStudentEnrollment[] }>(
          endpoint,
          token,
        );
        const enrollments = response.data ?? [];
        return mapStudentEnrollmentsResponse(enrollments);
      } else {
        throw new Error("Failed to update enrollments");
      }
    }
    // Regular single student enrollment
    else if (body.student_id) {
      const studentId = String(body.student_id);
      // Enroll a single student
      const result = await enrollStudent(id, { studentId });

      if (result.success) {
        // Return updated enrolled students - call backend directly with token
        const endpoint = `/api/activities/${id}/students`;
        const response = await apiGet<{ data: BackendStudentEnrollment[] }>(
          endpoint,
          token,
        );
        const enrollments = response.data ?? [];
        return mapStudentEnrollmentsResponse(enrollments);
      } else {
        throw new Error("Failed to enroll student");
      }
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
    const enrollments = response.data ?? [];
    // Map the backend enrollment structure to frontend format
    return mapStudentEnrollmentsResponse(enrollments);
  },
);
