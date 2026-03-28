import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  isStringParam,
  operatorApiGet,
} from "~/lib/operator/route-wrapper";
import { encodePathSegment } from "~/lib/route-wrapper-utils";

export const GET = createOperatorGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    const schoolId = encodePathSegment(params.id);
    return await operatorApiGet(`/operator/schools/${schoolId}/persons`, token);
  },
);
