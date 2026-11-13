import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

// Übernimmt eine Abwesenheit in Dienstplan und Betreuungsplan (#7021).
export const POST = createPostHandler<unknown, Record<string, never>>(
  async (
    _request: NextRequest,
    _body: Record<string, never>,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const absenceId = params.absenceId as string;
    if (!/^\d+$/.test(absenceId)) {
      throw new Error("Invalid absence id");
    }
    const response = await apiPost<{ data: unknown }>(
      `/api/staff/absences/${encodeURIComponent(absenceId)}/plan-sync`,
      token,
      {},
    );
    return response.data;
  },
);
