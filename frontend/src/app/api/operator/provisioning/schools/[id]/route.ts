import type { NextRequest } from "next/server";
import {
  createOperatorDeleteHandler,
  createOperatorPutHandler,
  isStringParam,
  operatorApiDelete,
  operatorApiPut,
} from "~/lib/operator/route-wrapper";

export const PUT = createOperatorPutHandler(
  async (_request: NextRequest, body: unknown, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    return await operatorApiPut(`/operator/schools/${params.id}`, token, body);
  },
);

export const DELETE = createOperatorDeleteHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    return await operatorApiDelete(`/operator/schools/${params.id}`, token);
  },
);
