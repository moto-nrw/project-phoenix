import type { NextRequest } from "next/server";
import {
  createOperatorDeleteHandler,
  isStringParam,
  operatorApiDelete,
} from "~/lib/operator/route-wrapper";

export const DELETE = createOperatorDeleteHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    return await operatorApiDelete(`/operator/invitations/${params.id}`, token);
  },
);
