import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  createOperatorProxyPostHandler,
  operatorApiGet,
} from "~/lib/operator/route-wrapper";

export const GET = createOperatorGetHandler(
  async (_request: NextRequest, token: string) => {
    return await operatorApiGet("/operator/invitations", token);
  },
);

export const POST = createOperatorProxyPostHandler("/operator/invitations");
