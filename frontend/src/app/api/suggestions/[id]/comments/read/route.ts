import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler, isStringParam } from "~/lib/route-wrapper";

export const POST = createPostHandler<null, Record<string, never>>(
  async (
    _request: NextRequest,
    _body: Record<string, never>,
    token: string,
    params: Record<string, unknown>,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid suggestion ID");
    }
    await apiPost(`/api/suggestions/${params.id}/comments/read`, token, {});
    return null;
  },
);
