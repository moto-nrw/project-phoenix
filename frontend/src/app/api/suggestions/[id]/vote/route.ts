// app/api/suggestions/[id]/vote/route.ts
import type { NextRequest } from "next/server";
import { apiPost, apiDelete } from "~/lib/api-helpers";
import {
  createPostHandler,
  createDeleteHandler,
  isStringParam,
} from "~/lib/route-wrapper";

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

export const POST = createPostHandler<BackendSuggestionResponse, VoteBody>(
  async (
    _request: NextRequest,
    body: VoteBody,
    token: string,
    params: Record<string, unknown>,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid suggestion ID");
    }
    const response = await apiPost<BackendSingleResponse>(
      `/api/suggestions/${params.id}/vote`,
      token,
      body,
    );
    return response.data;
  },
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
