// Settings sync API route
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

export const POST = createPostHandler(
  async (_request: NextRequest, _body: unknown, token: string) => {
    return await apiPost("/api/settings/sync", token, {});
  },
);
