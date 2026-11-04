import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy for GET /api/statistics/students/{id}/care-times (Soll und Ist je
 * Kind, #4774). The backend enforces users:read and the school boundary.
 */
export const GET = createGetHandler(
  async (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    if (!id) throw new Error("Student ID is required");
    const query = new URLSearchParams();
    for (const key of ["from", "to"] as const) {
      const value = request.nextUrl.searchParams.get(key);
      if (value) query.set(key, value);
    }
    const response = await apiGet<{ data: unknown }>(
      `/api/statistics/students/${encodeURIComponent(id)}/care-times?${query.toString()}`,
      token,
    );
    return response.data;
  },
);
