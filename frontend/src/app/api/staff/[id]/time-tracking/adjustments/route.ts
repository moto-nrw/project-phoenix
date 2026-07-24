import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { proxyGet } from "~/lib/route-proxy.server";
import { createPostHandler } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * GET /api/staff/[id]/time-tracking/adjustments?from=&to=
 * Stundenkonto transactions of one staff member (#1420). Gated server-side
 * on time_tracking:manage.
 */
export const GET = proxyGet(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/time-tracking/adjustments`,
);

// Explicit allowlist: only these fields reach the backend.
interface CreateAdjustmentBody {
  type?: string;
  minutes_delta?: number;
  effective_date?: string;
  note?: string;
}

/**
 * POST /api/staff/[id]/time-tracking/adjustments
 * Creates a payout or comp-time Stundenkonto transaction (#1420 5a).
 */
export const POST = createPostHandler<unknown, CreateAdjustmentBody>(
  async (
    _request: NextRequest,
    body: CreateAdjustmentBody,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const staffId = requirePathSegmentParam(params);
    const response = await apiPost<{ data: unknown }>(
      `/api/staff/${staffId}/time-tracking/adjustments`,
      token,
      {
        type: body.type,
        minutes_delta: body.minutes_delta,
        effective_date: body.effective_date,
        note: body.note,
      },
    );
    return response.data;
  },
);
