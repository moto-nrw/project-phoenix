// app/api/activities/[id]/students/[studentId]/route.ts
import type { NextRequest } from "next/server";
import { apiDelete } from "~/lib/api-helpers";
import { createDeleteHandler } from "~/lib/route-wrapper";

/**
 * Handler for DELETE /api/activities/[id]/students/[studentId]
 * Unenrolls a specific student from the activity
 */
export const DELETE = createDeleteHandler(
  async (_request: NextRequest, token: string, params: Record<string, unknown>) => {
    const id = params.id as string;
    const studentId = params.studentId as string;
    const endpoint = `/api/activities/${id}/students/${studentId}`;
    
    try {
      await apiDelete(endpoint, token);
      return { success: true };
    } catch (error) {
      console.error('Error unenrolling student:', error);
      throw error;
    }
  }
);