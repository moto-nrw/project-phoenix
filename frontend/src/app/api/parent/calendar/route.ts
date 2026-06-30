import type { NextRequest } from "next/server";
import {
  createParentGetHandler,
  parentApiGet,
} from "~/lib/parent/route-wrapper.server";

export const GET = createParentGetHandler(
  async (request: NextRequest, token: string) => {
    const search = request.nextUrl.searchParams.toString();
    return parentApiGet<unknown>(
      `/parent/me/calendar${search ? `?${search}` : ""}`,
      token,
    );
  },
);
