import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";

/**
 * GET /api/time-tracking/absences?from=YYYY-MM-DD&to=YYYY-MM-DD
 * Get staff absences for date range
 */
export const GET = createGetHandler<unknown>(
  async (request: NextRequest, token: string) => {
    const searchParams = request.nextUrl.searchParams;
    const from = searchParams.get("from") ?? "";
    const to = searchParams.get("to") ?? "";

    const params = new URLSearchParams();
    if (from) params.append("from", from);
    if (to) params.append("to", to);

    const queryString = params.toString();
    const endpoint = queryString
      ? `/api/time-tracking/absences?${queryString}`
      : "/api/time-tracking/absences";

    const response = await apiGet<{ data: unknown }>(endpoint, token);
    return response.data;
  },
);

interface CreateAbsenceBody {
  absence_type: string;
  date_start: string;
  date_end: string;
  half_day?: boolean;
  note?: string;
}

/**
 * POST /api/time-tracking/absences
 * Create a new absence
 */
export const POST = createPostHandler<unknown, CreateAbsenceBody>(
  async (_request: NextRequest, body: CreateAbsenceBody, token: string) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/time-tracking/absences",
      token,
      body,
    );
    return response.data;
  },
);
