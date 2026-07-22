import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

interface ResubmitBody {
  note?: string;
}

// Answer to a Rückfrage (#1419): the MA amends their note and moves the
// absence from "question" back to "requested".
export const POST = createPostHandler<unknown, ResubmitBody>(
  async (
    _request: NextRequest,
    body: ResubmitBody,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id;
    if (typeof id !== "string" || !/^\d+$/.test(id)) {
      throw new Error("Invalid absence id");
    }
    const response = await apiPost<{ data: unknown }>(
      `/api/time-tracking/absences/${id}/resubmit`,
      token,
      body ?? {},
    );
    return response.data;
  },
);
