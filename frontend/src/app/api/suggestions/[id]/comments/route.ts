import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import {
  createGetHandler,
  createPostHandler,
  isStringParam,
} from "~/lib/route-wrapper";

interface BackendCommentResponse {
  id: number;
  content: string;
  author_id: number;
  author_name: string;
  author_type: string;
  created_at: string;
}

interface BackendListResponse {
  status: string;
  data: BackendCommentResponse[];
}

interface CreateCommentRequest {
  content: string;
}

export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid suggestion ID");
    }
    const response = await apiGet<BackendListResponse>(
      `/api/suggestions/${params.id}/comments`,
      token,
    );
    return response.data;
  },
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
