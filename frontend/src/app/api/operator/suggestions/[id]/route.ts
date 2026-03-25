import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  createOperatorDeleteHandler,
  operatorApiGet,
  operatorApiDelete,
  isStringParam,
} from "~/lib/operator/route-wrapper";

export const GET = createOperatorGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    return await operatorApiGet(`/operator/suggestions/${params.id}`, token);
  },
);

export const DELETE = createOperatorDeleteHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    await operatorApiDelete(`/operator/suggestions/${params.id}`, token);
    return null;
  },
);
