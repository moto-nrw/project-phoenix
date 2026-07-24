// app/api/admin/grade-transitions/[id]/apply/route.ts
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
    const id = params.id;
    if (typeof id !== "string" || !/^\d+$/.test(id)) {
      throw new Error("Invalid id parameter");
    }
    const response = await apiPost<ResultResponse>(
      `/api/admin/grade-transitions/${id}/apply`,
      token,
      {},
    );
    return response.data;
  },
);
