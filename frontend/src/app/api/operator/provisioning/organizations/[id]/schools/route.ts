import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  isStringParam,
  operatorApiGet,
} from "~/lib/operator/route-wrapper.server";

export const GET = createOperatorGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    return await operatorApiGet(
      `/operator/organizations/${params.id}/schools`,
      token,
    );
  },
);
