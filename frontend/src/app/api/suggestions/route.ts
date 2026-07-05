// app/api/suggestions/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import { proxyPost } from "~/lib/route-proxy.server";

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

export const POST = proxyPost<BackendSuggestionResponse, CreateRequest>(
  "/api/suggestions",
);
