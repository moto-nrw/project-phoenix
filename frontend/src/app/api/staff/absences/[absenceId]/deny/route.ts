import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

interface DecisionBody {
  decision_note: string;
}

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
      `/api/staff/absences/${absenceId}/deny`,
      token,
      body,
    );
    return response.data;
  },
);
