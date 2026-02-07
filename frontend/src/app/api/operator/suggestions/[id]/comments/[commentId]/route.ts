import type { NextRequest } from "next/server";
import {
  createOperatorDeleteHandler,
  operatorApiDelete,
  isStringParam,
} from "~/lib/operator/route-wrapper";

export const DELETE = createOperatorDeleteHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id) || !isStringParam(params.commentId)) {
      throw new Error("Invalid parameters");
    }
    await operatorApiDelete(
      `/operator/suggestions/${params.id}/comments/${params.commentId}`,
      token,
    );
    return null;
  },
);
