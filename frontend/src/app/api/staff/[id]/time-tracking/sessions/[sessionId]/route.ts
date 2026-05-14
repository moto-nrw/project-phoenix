import type { NextRequest } from "next/server";
import { apiPut } from "~/lib/api-helpers";
import { createPutHandler } from "~/lib/route-wrapper";

/**
 * PUT /api/staff/[id]/time-tracking/sessions/[sessionId]
 * Admin edit endpoint: correct an existing work session for the named
 * staff member. Gated server-side on time_tracking:manage.
 */
export const PUT = createPutHandler<unknown, unknown>(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const sessionId = params.sessionId as string;
    const response = await apiPut<{ data: unknown }>(
      `/api/staff/${id}/time-tracking/sessions/${sessionId}`,
      token,
      body,
    );
    return response.data;
  },
);
