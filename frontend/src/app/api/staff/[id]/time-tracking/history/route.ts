import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * GET /api/staff/[id]/time-tracking/history?from=YYYY-MM-DD&to=YYYY-MM-DD
 * Admin endpoint: get work session history for a specific staff member
 */
export const GET = createGetHandler(
  async (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const searchParams = request.nextUrl.searchParams;
    const from = searchParams.get("from");
    const to = searchParams.get("to");

    const response = await apiGet<{ data: unknown }>(
      `/api/staff/${id}/time-tracking/history?from=${from}&to=${to}`,
      token,
    );
    return response.data;
  },
);
