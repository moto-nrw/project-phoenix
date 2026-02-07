import type { NextRequest } from "next/server";
import { apiDelete } from "~/lib/api-helpers";
import { createDeleteHandler, isStringParam } from "~/lib/route-wrapper";

export const DELETE = createDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    if (!isStringParam(params.id) || !isStringParam(params.commentId)) {
      throw new Error("Invalid suggestion or comment ID");
    }
    await apiDelete(
      `/api/suggestions/${params.id}/comments/${params.commentId}`,
      token,
    );
    return null;
  },
);
