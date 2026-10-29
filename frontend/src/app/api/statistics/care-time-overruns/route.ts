import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import { statisticsQuery } from "~/lib/statistics-download-proxy.server";

/**
 * Proxy for GET /api/statistics/care-time-overruns (Soll und Ist, #4774).
 * The backend enforces config:read + users:read and writes the data-access
 * audit row; only the window and the group and class filters are forwarded.
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const response = await apiGet<{ data: unknown }>(
      `/api/statistics/care-time-overruns${statisticsQuery(request, ["from", "to"])}`,
      token,
    );
    return response.data;
  },
);
