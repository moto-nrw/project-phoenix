import type { NextRequest } from "next/server";
import {
  createOperatorDeleteHandler,
  isStringParam,
  operatorApiDelete,
} from "~/lib/operator/route-wrapper";
import { encodePathSegment } from "~/lib/route-wrapper-utils";

export const DELETE = createOperatorDeleteHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    const personId = encodePathSegment(params.id);
    return await operatorApiDelete(`/operator/persons/${personId}`, token);
  },
);
