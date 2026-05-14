import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

/**
 * GET /api/staff/[id]/absences?from=YYYY-MM-DD&to=YYYY-MM-DD
 * Admin endpoint: list absences (Krank/Urlaub/etc.) for a specific staff
 * member. Mirrors /api/staff/[id]/time-tracking/history so the staff detail
 * view can render absence rows next to work sessions.
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
      `/api/staff/${id}/absences?from=${from}&to=${to}`,
      token,
    );
    return response.data;
  },
);
