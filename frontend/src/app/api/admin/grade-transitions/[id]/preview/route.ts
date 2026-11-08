// app/api/admin/grade-transitions/[id]/preview/route.ts
import { requireNumericParam } from "~/lib/route-wrapper-utils.server";
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
    const id = requireNumericParam(params);
    const response = await apiGet<PreviewResponse>(
      `/api/admin/grade-transitions/${id}/preview`,
      token,
    );
    return response.data;
  },
);
