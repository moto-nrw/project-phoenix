import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Explicit allowlist: only these fields reach the backend.
interface OpeningBalanceBody {
  effective_date?: string;
  balance_minutes?: number;
  note?: string;
}

/**
 * POST /api/staff/[id]/time-tracking/opening
 * Go-live Eröffnungssaldo (#2132): the backend computes the delta that turns
 * the closing balance as of effective_date into the SIGNED target value.
 */
export const POST = createPostHandler<unknown, OpeningBalanceBody>(
  async (
    _request: NextRequest,
    body: OpeningBalanceBody,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const staffId = requirePathSegmentParam(params);
    const response = await apiPost<{ data: unknown }>(
      `/api/staff/${staffId}/time-tracking/opening`,
      token,
      {
        effective_date: body.effective_date,
        balance_minutes: body.balance_minutes,
        note: body.note,
      },
    );
    return response.data;
  },
);
