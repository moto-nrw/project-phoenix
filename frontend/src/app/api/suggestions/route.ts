// app/api/suggestions/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";

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

interface BackendListResponse {
  status: string;
  data: BackendSuggestionResponse[];
}

interface BackendSingleResponse {
  status: string;
  data: BackendSuggestionResponse;
}

interface CreateRequest {
  title: string;
  description: string;
}

export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const sort = request.nextUrl.searchParams.get("sort") ?? "score";
    const response = await apiGet<BackendListResponse>(
      `/api/suggestions?sort=${sort}`,
      token,
    );
    // Return inner data — the route wrapper adds its own { success, data } envelope
    return response.data;
  },
);

export const POST = createPostHandler<BackendSuggestionResponse, CreateRequest>(
  async (_request: NextRequest, body: CreateRequest, token: string) => {
    const response = await apiPost<BackendSingleResponse>(
      "/api/suggestions",
      token,
      body,
    );
    return response.data;
  },
);
