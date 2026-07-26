import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  operatorApiGet,
  proxyPost,
} from "~/lib/operator/route-wrapper.server";

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

export const POST = proxyPost("/operator/announcements");
