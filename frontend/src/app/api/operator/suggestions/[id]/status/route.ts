import type { NextRequest } from "next/server";
import {
  createOperatorPutHandler,
  operatorApiPut,
  isStringParam,
} from "~/lib/operator/route-wrapper";

export const PUT = createOperatorPutHandler(
  async (_request: NextRequest, body: unknown, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    return await operatorApiPut(
      `/operator/suggestions/${params.id}/status`,
      token,
      body,
    );
  },
);
