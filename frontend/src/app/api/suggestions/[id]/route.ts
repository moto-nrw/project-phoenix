// app/api/suggestions/[id]/route.ts
import { proxyDelete, proxyGet, proxyPut } from "~/lib/route-proxy.server";
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

interface UpdateRequest {
  title: string;
  description: string;
}

export const GET = proxyGet<BackendSuggestionResponse>(
  (params) => `/api/suggestions/${requirePathSegmentParam(params)}`,
);

export const PUT = proxyPut<BackendSuggestionResponse, UpdateRequest>(
  (params) => `/api/suggestions/${requirePathSegmentParam(params)}`,
);

export const DELETE = proxyDelete(
  (params) => `/api/suggestions/${requirePathSegmentParam(params)}`,
);
