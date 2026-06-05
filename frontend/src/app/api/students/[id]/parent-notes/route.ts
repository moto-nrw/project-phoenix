import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/students/{id}/parent-notes → backend. Returns the newest
 * notes a guardian left for the student via the parents portal. Backend
 * enforces staff read access to the student.
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    if (!id) {
      throw new Error("Student ID is required");
    }
    const response = await apiGet<{ data: unknown }>(
      `/api/students/${id}/parent-notes`,
      token,
    );
    return response.data;
  },
);
