// POST /api/timetable/instances/[id]/cancel — cancels a planned or active
// instance; from active, ends the bridge cleanly via
// active.EndActivitySession.
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler, isStringParam } from "~/lib/route-wrapper";

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
      `/api/timetable/instances/${params.id}/cancel`,
      token,
      body ?? {},
    );
    return response.data;
  },
);
