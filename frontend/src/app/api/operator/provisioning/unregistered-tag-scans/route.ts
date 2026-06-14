import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  operatorApiGet,
} from "~/lib/operator/route-wrapper.server";

export const GET = createOperatorGetHandler(
  async (request: NextRequest, token: string) => {
    const query = request.nextUrl.search;
    return await operatorApiGet(
      `/operator/unregistered-tag-scans${query}`,
      token,
    );
  },
);
