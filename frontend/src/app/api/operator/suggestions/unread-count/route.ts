import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  operatorApiGet,
} from "~/lib/operator/route-wrapper.server";

export const GET = createOperatorGetHandler(
  async (_request: NextRequest, token: string) => {
    return await operatorApiGet("/operator/suggestions/unread-count", token);
  },
);
