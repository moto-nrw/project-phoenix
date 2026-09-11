import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

// Creates a fresh staff calendar subscription link and invalidates the old one.
export const POST = createPostHandler(
  async (_request: NextRequest, _body: unknown, token: string) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/calendar/feed/rotate",
      token,
      {},
    );
    return response.data;
  },
);
