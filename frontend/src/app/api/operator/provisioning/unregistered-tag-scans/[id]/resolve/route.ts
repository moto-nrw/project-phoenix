import type { NextRequest } from "next/server";
import {
  createOperatorPostHandler,
  isStringParam,
  operatorApiPost,
} from "~/lib/operator/route-wrapper.server";
import { encodePathSegment } from "~/lib/route-wrapper-utils.server";

export const POST = createOperatorPostHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    const scanId = encodePathSegment(params.id);
    return await operatorApiPost(
      `/operator/unregistered-tag-scans/${scanId}/resolve`,
      token,
      body,
    );
  },
);
