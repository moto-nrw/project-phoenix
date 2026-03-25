import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  createOperatorPostHandler,
  isStringParam,
  operatorApiGet,
  operatorApiPost,
} from "~/lib/operator/route-wrapper";

export const GET = createOperatorGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    return await operatorApiGet(
      `/operator/schools/${params.id}/accounts`,
      token,
    );
  },
);

export const POST = createOperatorPostHandler(
  async (_request: NextRequest, body: unknown, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    return await operatorApiPost(
      `/operator/schools/${params.id}/accounts`,
      token,
      body,
    );
  },
);
