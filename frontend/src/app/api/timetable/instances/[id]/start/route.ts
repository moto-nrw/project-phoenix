// POST /api/timetable/instances/[id]/start — transitions a planned
// instance to active and creates the active.group bridge.
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler, isStringParam } from "~/lib/route-wrapper.server";

export const POST = createPostHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    // Go backend returns { status, data, message }; strip the envelope.
    const response = await apiPost<{ data: unknown }>(
      `/api/timetable/instances/${params.id}/start`,
      token,
      body ?? {},
    );
    return response.data;
  },
);
