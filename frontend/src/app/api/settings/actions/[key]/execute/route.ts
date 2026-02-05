// Settings action execute API route
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler, isStringParam } from "~/lib/route-wrapper";

export const POST = createPostHandler(
  async (
    _request: NextRequest,
    _body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    if (!isStringParam(params.key)) {
      throw new Error("Action key is required");
    }
    return await apiPost(
      `/api/settings/actions/${encodeURIComponent(params.key)}/execute`,
      token,
      {},
    );
  },
);
