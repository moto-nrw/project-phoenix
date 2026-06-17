import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/messages/students/{studentId}/guardians → backend. Returns
 * the child's guardians who have a parents-portal account and can therefore
 * receive a new message thread.
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
      `/api/messages/students/${studentId}/guardians`,
      token,
    );
    return response.data;
  },
);
