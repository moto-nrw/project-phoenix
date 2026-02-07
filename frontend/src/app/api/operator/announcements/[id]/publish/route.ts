import type { NextRequest } from "next/server";
import {
  createOperatorPostHandler,
  operatorApiPost,
  isStringParam,
} from "~/lib/operator/route-wrapper";

export const POST = createOperatorPostHandler(
  async (_request: NextRequest, _body: unknown, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    return await operatorApiPost(
      `/operator/announcements/${params.id}/publish`,
      token,
    );
  },
);
