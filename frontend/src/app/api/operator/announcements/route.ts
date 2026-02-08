import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  createOperatorPostHandler,
  operatorApiGet,
  operatorApiPost,
} from "~/lib/operator/route-wrapper";

export const GET = createOperatorGetHandler(
  async (request: NextRequest, token: string) => {
    const includeInactive =
      request.nextUrl.searchParams.get("include_inactive") ?? "true";
    return await operatorApiGet(
      `/operator/announcements?include_inactive=${includeInactive}`,
      token,
    );
  },
);

export const POST = createOperatorPostHandler(
  async (_request: NextRequest, body: unknown, token: string) => {
    return await operatorApiPost("/operator/announcements", token, body);
  },
);
