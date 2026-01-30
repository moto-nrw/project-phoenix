// app/api/suggestions/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers";
import {
  createGetHandler,
  createPutHandler,
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

interface UpdateRequest {
  title: string;
  description: string;
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
    const response = await apiGet<BackendSingleResponse>(
      `/api/suggestions/${params.id}`,
      token,
    );
    return response.data;
  },
);

export const PUT = createPutHandler<BackendSuggestionResponse, UpdateRequest>(
  async (
    _request: NextRequest,
    body: UpdateRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid suggestion ID");
    }
    const response = await apiPut<BackendSingleResponse>(
      `/api/suggestions/${params.id}`,
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
    await apiDelete(`/api/suggestions/${params.id}`, token);
    return null;
  },
);
