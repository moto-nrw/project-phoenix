import type { NextRequest } from "next/server";
import {
  createOperatorPutHandler,
  isStringParam,
  operatorApiPut,
} from "~/lib/operator/route-wrapper";

export const PUT = createOperatorPutHandler(
  async (_request: NextRequest, body: unknown, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    return await operatorApiPut(
      `/operator/organizations/${params.id}`,
      token,
      body,
    );
  },
);
