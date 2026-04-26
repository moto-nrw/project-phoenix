// POST /api/timetable/instances/[id]/complete — transitions an active
// instance to completed; closes the active.group, marks remaining expected
// students as absent.
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
      `/api/timetable/instances/${params.id}/complete`,
      token,
      body ?? {},
    );
    return response.data;
  },
);
