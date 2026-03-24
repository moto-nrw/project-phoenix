import type { NextRequest } from "next/server";
import {
  createOperatorPostHandler,
  operatorApiPost,
} from "~/lib/operator/route-wrapper";

export const POST = createOperatorPostHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    return await operatorApiPost(
      `/operator/devices/${String(params.id)}/set-api-key`,
      token,
      body,
    );
  },
);
