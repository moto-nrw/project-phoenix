// app/api/suggestions/[id]/vote/route.ts
import type { NextRequest } from "next/server";
import { apiDelete } from "~/lib/api-helpers.server";
import { createDeleteHandler, isStringParam } from "~/lib/route-wrapper.server";
import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface BackendSuggestionResponse {
  id: number;
  title: string;
  description: string;
  author_id: number;
  author_name: string;
  status: string;
  score: number;
  user_vote: string | null;
  created_at: string;
  updated_at: string;
}

interface BackendSingleResponse {
  status: string;
  data: BackendSuggestionResponse;
}

interface VoteBody {
  direction: "up" | "down";
}

export const POST = proxyPost<BackendSuggestionResponse, VoteBody>(
  (params) => `/api/suggestions/${requirePathSegmentParam(params)}/vote`,
);

export const DELETE = createDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid suggestion ID");
    }
    const response = await apiDelete<BackendSingleResponse>(
      `/api/suggestions/${params.id}/vote`,
      token,
    );
    return response ? response.data : null;
  },
);
