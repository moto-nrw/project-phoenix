import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

interface DecisionBody {
  decision_note?: string;
}

// Rückfrage (#1419): moves a requested absence into status "question" with a
// mandatory note from the Leitung.
export const POST = createPostHandler<unknown, DecisionBody>(
  async (
    _request: NextRequest,
    body: DecisionBody,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const absenceId = params.absenceId as string;
    if (!/^\d+$/.test(absenceId)) {
      throw new Error("Invalid absence id");
    }
    const response = await apiPost<{ data: unknown }>(
      `/api/staff/absences/${absenceId}/question`,
      token,
      body ?? {},
    );
    return response.data;
  },
);
