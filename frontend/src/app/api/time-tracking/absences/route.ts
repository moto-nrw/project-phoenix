import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";
import { createProxyGetDataHandler } from "~/lib/route-wrapper";

/**
 * GET /api/time-tracking/absences?from=YYYY-MM-DD&to=YYYY-MM-DD
 * Get staff absences for date range
 */
export const GET = createProxyGetDataHandler("/api/time-tracking/absences");

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
