import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Explicit allowlist: only these fields reach the backend.
interface ResetBalanceBody {
  effective_date?: string;
  carryover_minutes?: number;
  note?: string;
}

/**
 * POST /api/staff/[id]/time-tracking/reset
 * School-year Stundenkonto reset (#1420 5c): the backend computes the
 * closing balance as of effective_date and writes the inverting transaction.
 */
export const POST = createPostHandler<unknown, ResetBalanceBody>(
  async (
    _request: NextRequest,
    body: ResetBalanceBody,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const staffId = requirePathSegmentParam(params);
    const response = await apiPost<{ data: unknown }>(
      `/api/staff/${staffId}/time-tracking/reset`,
      token,
      {
        effective_date: body.effective_date,
        carryover_minutes: body.carryover_minutes,
        note: body.note,
      },
    );
    return response.data;
  },
);
