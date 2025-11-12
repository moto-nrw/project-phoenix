// app/api/activities/[id]/students/[studentId]/route.ts
import type { NextRequest } from "next/server";
import { createGetHandler, createDeleteHandler } from "~/lib/route-wrapper";
import { apiDelete } from "~/lib/api-helpers";
import { getEnrolledStudents } from "~/lib/activity-api";

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

    // Get all enrolled students for the activity
    const students = await getEnrolledStudents(id);

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

    try {
      // Call backend directly to unenroll the student
      const endpoint = `/api/activities/${id}/students/${studentId}`;
      await apiDelete(endpoint, token);

      return { success: true };
    } catch (error) {
      throw error;
    }
  },
);
