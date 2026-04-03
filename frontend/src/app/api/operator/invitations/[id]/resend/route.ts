import type { NextRequest } from "next/server";
import {
  createOperatorPostHandler,
  operatorApiPost,
} from "~/lib/operator/route-wrapper";

export const POST = createOperatorPostHandler(
  async (_request: NextRequest, _body: unknown, token: string, params) => {
    return await operatorApiPost(
      `/operator/invitations/${String(params.id)}/resend`,
      token,
      {},
    );
  },
);
