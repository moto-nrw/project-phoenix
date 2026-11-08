import type { NextRequest } from "next/server";
import {
  createOperatorPostHandler,
  operatorApiPost,
} from "~/lib/operator/route-wrapper.server";
import {
  encodePathSegment,
  requireStringParam,
} from "~/lib/route-wrapper-utils.server";

export const POST = createOperatorPostHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = requireStringParam(params, "id");
    const scanId = encodePathSegment(id);
    return await operatorApiPost(
      `/operator/unregistered-tag-scans/${scanId}/resolve`,
      token,
      body,
    );
  },
);
