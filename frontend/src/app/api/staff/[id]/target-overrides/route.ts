import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { proxyGet } from "~/lib/route-proxy.server";
import { createPostHandler } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * GET /api/staff/[id]/target-overrides
 * Sonderarbeitszeiten of one staff member (#3259). Gated server-side on
 * time_tracking:manage.
 */
export const GET = proxyGet(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/target-overrides`,
);

// Explicit allowlist: only these fields reach the backend.
interface TargetOverrideBody {
  start_date?: string;
  end_date?: string;
  daily_minutes?: number;
}

/**
 * POST /api/staff/[id]/target-overrides
 * Creates a Sonderarbeitszeit: a date range with one daily target.
 */
export const POST = createPostHandler<unknown, TargetOverrideBody>(
  async (
    _request: NextRequest,
    body: TargetOverrideBody,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const staffId = requirePathSegmentParam(params);
    const response = await apiPost<{ data: unknown }>(
      `/api/staff/${staffId}/target-overrides`,
      token,
      {
        start_date: body.start_date,
        end_date: body.end_date,
        daily_minutes: body.daily_minutes,
      },
    );
    return response.data;
  },
);
