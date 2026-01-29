// Settings value restore API route
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler, isStringParam } from "~/lib/route-wrapper";

interface RestoreValueBody {
  scope: string;
  scope_id?: number;
}

export const POST = createPostHandler<unknown, RestoreValueBody>(
  async (
    _request: NextRequest,
    body: RestoreValueBody,
    token: string,
    params,
  ) => {
    if (!isStringParam(params.key)) {
      throw new Error("Invalid key parameter");
    }

    return await apiPost(
      `/api/settings/values/${encodeURIComponent(params.key)}/restore`,
      token,
      body,
    );
  },
);
