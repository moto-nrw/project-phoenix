import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy for GET /api/statistics/care-time-overruns (Soll und Ist, #4774).
 * The backend enforces config:read + users:read and writes the data-access
 * audit row; only the window and the group filter are forwarded.
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const params = new URLSearchParams();
    for (const key of ["from", "to"] as const) {
      const value = request.nextUrl.searchParams.get(key);
      if (value) params.set(key, value);
    }
    for (const groupId of request.nextUrl.searchParams.getAll("group_id")) {
      if (groupId) params.append("group_id", groupId);
    }
    const response = await apiGet<{ data: unknown }>(
      `/api/statistics/care-time-overruns?${params.toString()}`,
      token,
    );
    return response.data;
  },
);
