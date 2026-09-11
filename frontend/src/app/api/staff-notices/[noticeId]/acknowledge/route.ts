import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

/** Proxy POST /api/staff-notices/{id}/acknowledge → Backend (Kenntnisnahme). */
export const POST = createPostHandler(
  async (
    _request: NextRequest,
    _body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.noticeId as string;
    if (!id) throw new Error("Notice ID is required");
    const response = await apiPost<{ data: unknown }>(
      `/api/staff-notices/${id}/acknowledge`,
      token,
      {},
    );
    return response?.data ?? null;
  },
);
