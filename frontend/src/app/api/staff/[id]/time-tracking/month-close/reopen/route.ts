import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Explicit allowlist: only these fields reach the backend.
interface MonthReopenBody {
  year?: number;
  month?: number;
  reason?: string;
}

/**
 * POST /api/staff/[id]/time-tracking/month-close/reopen
 * Reopens one staff member's closed month (#1417). Reason is mandatory
 * backend-side; reopening proceeds newest-first per person.
 */
export const POST = createPostHandler<unknown, MonthReopenBody>(
  async (
    _request: NextRequest,
    body: MonthReopenBody,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const staffId = requirePathSegmentParam(params);
    const response = await apiPost<{ data: unknown }>(
      `/api/staff/${staffId}/time-tracking/month-close/reopen`,
      token,
      {
        year: body.year,
        month: body.month,
        reason: body.reason,
      },
    );
    return response.data;
  },
);
