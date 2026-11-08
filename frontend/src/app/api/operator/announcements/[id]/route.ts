import type { NextRequest } from "next/server";
import {
  createOperatorDeleteHandler,
  operatorApiDelete,
  proxyGet,
  proxyPut,
} from "~/lib/operator/route-wrapper.server";
import {
  encodePathSegment,
  requirePathSegmentParam,
  requireStringParam,
} from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (params) => `/operator/announcements/${requirePathSegmentParam(params)}`,
);

export const PUT = proxyPut(
  (params) => `/operator/announcements/${requirePathSegmentParam(params)}`,
);

export const DELETE = createOperatorDeleteHandler(
  async (_request: NextRequest, token: string, params) => {
    const id = requireStringParam(params, "id");
    return await operatorApiDelete(
      `/operator/announcements/${encodePathSegment(id)}`,
      token,
    );
  },
);
