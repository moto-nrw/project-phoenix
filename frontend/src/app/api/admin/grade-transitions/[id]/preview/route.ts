// app/api/admin/grade-transitions/[id]/preview/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import type { BackendTransitionPreview } from "~/lib/grade-transition-api";

interface PreviewResponse {
  data: BackendTransitionPreview;
}

export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token,
    params,
  ): Promise<BackendTransitionPreview> => {
    const id = params.id;
    if (typeof id !== "string" || !/^\d+$/.test(id)) {
      throw new Error("Invalid id parameter");
    }
    const response = await apiGet<PreviewResponse>(
      `/api/admin/grade-transitions/${id}/preview`,
      token,
    );
    return response.data;
  },
);
