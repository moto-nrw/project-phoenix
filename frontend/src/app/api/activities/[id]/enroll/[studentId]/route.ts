// app/api/activities/[id]/enroll/[studentId]/route.ts
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

/**
 * Handler for POST /api/activities/[id]/enroll/[studentId]
 * Enrolls a student in an activity
 */
export const POST = createPostHandler<{ success: boolean }, {}>(
  async (_request: NextRequest, _body: {}, token: string, params: Record<string, unknown>) => {
    const id = params.id as string;
    const studentId = params.studentId as string;
    
    // Match the backend endpoint format
    const endpoint = `/api/activities/${id}/enroll/${studentId}`;
    
    try {
      // Empty body since the backend doesn't expect one
      await apiPost(endpoint, token, {});
      return { success: true };
    } catch (error) {
      console.error('Error enrolling student:', error);
      throw error;
    }
  }
);