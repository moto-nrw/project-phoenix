// app/api/admin/grade-transitions/route.ts
// Proxy for the Jahrgangsstufenwechsel admin endpoints (#405).
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";
import type { BackendGradeTransition } from "~/lib/grade-transition-api";

interface ListResponse {
  data: BackendGradeTransition[];
}

interface DetailResponse {
  data: BackendGradeTransition;
}

export const GET = createGetHandler(
  async (request: NextRequest, token): Promise<BackendGradeTransition[]> => {
    const search = request.nextUrl.searchParams;
    const query = new URLSearchParams();
    for (const key of ["page", "page_size", "status", "academic_year"]) {
      const value = search.get(key);
      if (value) query.set(key, value);
    }
    const suffix = query.size > 0 ? `?${query.toString()}` : "";
    const response = await apiGet<ListResponse>(
      `/api/admin/grade-transitions${suffix}`,
      token,
    );
    return response.data ?? [];
  },
);

export const POST = createPostHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token,
  ): Promise<BackendGradeTransition> => {
    const response = await apiPost<DetailResponse>(
      "/api/admin/grade-transitions",
      token,
      body,
    );
    return response.data;
  },
);
