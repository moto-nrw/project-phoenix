import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  operatorApiGet,
} from "~/lib/operator/route-wrapper.server";
import {
  encodePathSegment,
  requireStringParam,
} from "~/lib/route-wrapper-utils.server";

export const GET = createOperatorGetHandler(
  async (_request: NextRequest, token: string, params) => {
    const id = requireStringParam(params, "id");
    const schoolId = encodePathSegment(id);
    return await operatorApiGet(`/operator/schools/${schoolId}/persons`, token);
  },
);
