// app/api/admin/grade-transitions/[id]/revert/route.ts
import { requireNumericParam } from "~/lib/route-wrapper-utils.server";
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";
import type { BackendTransitionResult } from "~/lib/grade-transition-api";

interface ResultResponse {
  data: BackendTransitionResult;
}

export const POST = createPostHandler(
  async (
    _request: NextRequest,
    _body: unknown,
    token,
    params,
  ): Promise<BackendTransitionResult> => {
    const id = requireNumericParam(params);
    const response = await apiPost<ResultResponse>(
      `/api/admin/grade-transitions/${id}/revert`,
      token,
      {},
    );
    return response.data;
  },
);
