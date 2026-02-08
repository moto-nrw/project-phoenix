import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  operatorApiGet,
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
