import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  createOperatorPostHandler,
  operatorApiGet,
  operatorApiPost,
} from "~/lib/operator/route-wrapper";

export const GET = createOperatorGetHandler(
  async (_request: NextRequest, token: string) => {
    return await operatorApiGet("/operator/organizations", token);
  },
);

export const POST = createOperatorPostHandler(
  async (_request: NextRequest, body: unknown, token: string) => {
    return await operatorApiPost("/operator/organizations", token, body);
  },
);
