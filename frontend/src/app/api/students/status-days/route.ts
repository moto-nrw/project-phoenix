import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

// Abwesenheits-Übersicht (#2288): forward-looking status-day list across all
// children of the caller's permitted groups.
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const query = request.nextUrl.search;
    const response = await apiGet<{ data: unknown }>(
      `/api/students/status-days${query}`,
      token,
    );
    return response.data;
  },
);
