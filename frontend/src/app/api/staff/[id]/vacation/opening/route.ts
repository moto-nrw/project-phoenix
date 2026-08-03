import type { NextRequest } from "next/server";
import { apiDelete, apiPost } from "~/lib/api-helpers.server";
import {
  createDeleteHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Explicit allowlist: only these fields reach the backend.
interface VacationOpeningBody {
  effective_date?: string;
  remaining_days?: number;
  note?: string;
}

/**
 * POST /api/staff/[id]/vacation/opening
 * Vacation takeover (#2132): the admin enters the Resturlaub as of the
 * Stichtag; the backend derives the pre-introduction days from the quota.
 */
export const POST = createPostHandler<unknown, VacationOpeningBody>(
  async (
    _request: NextRequest,
    body: VacationOpeningBody,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const staffId = requirePathSegmentParam(params);
    const response = await apiPost<{ data: unknown }>(
      `/api/staff/${staffId}/vacation/opening`,
      token,
      {
        effective_date: body.effective_date,
        remaining_days: body.remaining_days,
        note: body.note,
      },
    );
    return response.data;
  },
);

/**
 * DELETE /api/staff/[id]/vacation/opening?year=YYYY
 * Removes the takeover (with an append-only deletion tombstone backend-side).
 */
export const DELETE = createDeleteHandler(
  async (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const staffId = requirePathSegmentParam(params);
    const year = request.nextUrl.searchParams.get("year") ?? "";
    if (year && !/^\d{4}$/.test(year)) {
      throw new Error("Invalid year");
    }
    const qs = year ? `?year=${year}` : "";
    await apiDelete(`/api/staff/${staffId}/vacation/opening${qs}`, token);
    return null;
  },
);
