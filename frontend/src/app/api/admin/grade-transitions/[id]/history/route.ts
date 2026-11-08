// app/api/admin/grade-transitions/[id]/history/route.ts
import { requireNumericParam } from "~/lib/route-wrapper-utils.server";
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import type { BackendTransitionHistoryEntry } from "~/lib/grade-transition-api";

interface HistoryResponse {
  data: BackendTransitionHistoryEntry[];
}

export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token,
    params,
  ): Promise<BackendTransitionHistoryEntry[]> => {
    const id = requireNumericParam(params);
    const response = await apiGet<HistoryResponse>(
      `/api/admin/grade-transitions/${id}/history`,
      token,
    );
    return response.data ?? [];
  },
);
