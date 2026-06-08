import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * GET /api/staff/[id]/time-tracking/sessions/[sessionId]/edits
 * Admin endpoint: list audit edits for a single work session of a staff
 * member. Mirrors /api/time-tracking/[id]/edits (the MA-self route),
 * but verifies the session belongs to the staff named in the URL
 * instead of the JWT subject.
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const sessionId = params.sessionId as string;

    const response = await apiGet<{ data: unknown }>(
      `/api/staff/${id}/time-tracking/sessions/${sessionId}/edits`,
      token,
    );
    return response.data;
  },
);
