// app/api/activities/[id]/students/[studentId]/route.ts
import type { NextRequest } from "next/server";
import {
  createGetHandler,
  createDeleteHandler,
} from "~/lib/route-wrapper.server";
import { apiDelete, apiGet } from "~/lib/api-helpers.server";
import {
  type BackendStudentEnrollment,
  mapStudentEnrollmentsResponse,
} from "~/lib/activity-helpers";

/**
 * Handler for GET /api/activities/[id]/students/[studentId]
 * Returns details about a specific student's enrollment
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const studentId = params.studentId as string;

    if (!id || !studentId) {
      throw new Error("Activity ID and Student ID are required");
    }

    const response = await apiGet<{ data: BackendStudentEnrollment[] }>(
      `/api/activities/${id}/students`,
      token,
    );
    const students = mapStudentEnrollmentsResponse(response.data ?? []);

    // Find the specific student
    const student = students.find((s) => s.student_id === studentId);

    if (!student) {
      throw new Error(
        `Student with ID ${studentId} is not enrolled in activity ${id}`,
      );
    }

    return student;
  },
);

/**
 * Handler for DELETE /api/activities/[id]/students/[studentId]
 * Unenrolls a student from an activity
 */
export const DELETE = createDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const studentId = params.studentId as string;

    if (!id || !studentId) {
      throw new Error("Activity ID and Student ID are required");
    }

    // Call backend directly to unenroll the student
    const endpoint = `/api/activities/${id}/students/${studentId}`;
    await apiDelete(endpoint, token);

    return { success: true };
  },
);
