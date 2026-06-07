import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

export const POST = createPostHandler(
  async (
    _request: NextRequest,
    _body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id;
    if (typeof id !== "string" || !/^\d+$/.test(id)) {
      throw new Error("Invalid absence id");
    }
    const response = await apiPost<{ data: unknown }>(
      `/api/time-tracking/absences/${id}/cancel`,
      token,
      {},
    );
    return response.data;
  },
);
