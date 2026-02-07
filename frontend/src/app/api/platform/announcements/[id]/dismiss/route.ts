import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler, isStringParam } from "~/lib/route-wrapper";

export const POST = createPostHandler<null, Record<string, never>>(
  async (_request: NextRequest, _body, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid announcement ID");
    }
    await apiPost(
      `/api/platform/announcements/${params.id}/dismiss`,
      token,
      {},
    );
    return null;
  },
);
