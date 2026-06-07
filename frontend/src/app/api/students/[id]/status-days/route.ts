import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";

export const GET = createGetHandler(
  async (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    if (!id) {
      throw new Error("Student ID is required");
    }
    const query = request.nextUrl.search;
    const response = await apiGet<{ data: unknown }>(
      `/api/students/${id}/status-days${query}`,
      token,
    );
    return response.data;
  },
);

export const POST = createPostHandler<
  unknown,
  { status: string; dates: string[] }
>(
  async (
    _request: NextRequest,
    body: { status: string; dates: string[] },
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    if (!id) {
      throw new Error("Student ID is required");
    }
    const response = await apiPost<
      { data: unknown },
      { status: string; dates: string[] }
    >(`/api/students/${id}/status-days`, token, body);
    return response.data;
  },
);
