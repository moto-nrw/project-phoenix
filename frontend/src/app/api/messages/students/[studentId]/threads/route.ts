import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/messages/students/{studentId}/threads → backend. Returns the
 * staff view of one child's conversations, so the student-detail card loads
 * only that child's threads instead of the whole tenant inbox.
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const studentId = params.studentId as string;
    if (!studentId) {
      throw new Error("Student ID is required");
    }
    const response = await apiGet<{ data: unknown }>(
      `/api/messages/students/${studentId}/threads`,
      token,
    );
    return response.data;
  },
);
