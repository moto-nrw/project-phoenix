import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler, isStringParam } from "~/lib/route-wrapper.server";
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface BackendCommentResponse {
  id: number;
  content: string;
  author_id: number;
  author_name: string;
  author_type: string;
  created_at: string;
}

interface CreateCommentRequest {
  content: string;
}

export const GET = proxyGet<BackendCommentResponse[]>(
  (params) => `/api/suggestions/${requirePathSegmentParam(params)}/comments`,
);

export const POST = createPostHandler<
  BackendCommentResponse | null,
  CreateCommentRequest
>(
  async (
    _request: NextRequest,
    body: CreateCommentRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid suggestion ID");
    }
    await apiPost(`/api/suggestions/${params.id}/comments`, token, body);
    return null;
  },
);
