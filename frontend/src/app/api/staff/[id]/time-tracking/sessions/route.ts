import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

/**
 * POST /api/staff/[id]/time-tracking/sessions
 * Admin "nachtragen" endpoint: create a work session for the named staff
 * member. Gated server-side on time_tracking:manage.
 */
export const POST = createPostHandler<unknown, unknown>(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const response = await apiPost<{ data: unknown }>(
      `/api/staff/${id}/time-tracking/sessions`,
      token,
      body,
    );
    return response.data;
  },
);
