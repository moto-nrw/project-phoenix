import type { NextRequest } from "next/server";
import {
  createOperatorDeleteHandler,
  isStringParam,
  operatorApiDelete,
  proxyPut,
} from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const PUT = proxyPut(
  (params) => `/operator/schools/${requirePathSegmentParam(params)}`,
);

export const DELETE = createOperatorDeleteHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    return await operatorApiDelete(`/operator/schools/${params.id}`, token);
  },
);
