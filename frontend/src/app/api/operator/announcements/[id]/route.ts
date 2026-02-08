import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  createOperatorPutHandler,
  createOperatorDeleteHandler,
  operatorApiGet,
  operatorApiPut,
  operatorApiDelete,
  isStringParam,
} from "~/lib/operator/route-wrapper";

export const GET = createOperatorGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    return await operatorApiGet(`/operator/announcements/${params.id}`, token);
  },
);

export const PUT = createOperatorPutHandler(
  async (_request: NextRequest, body: unknown, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    return await operatorApiPut(
      `/operator/announcements/${params.id}`,
      token,
      body,
    );
  },
);

export const DELETE = createOperatorDeleteHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    return await operatorApiDelete(
      `/operator/announcements/${params.id}`,
      token,
    );
  },
);
