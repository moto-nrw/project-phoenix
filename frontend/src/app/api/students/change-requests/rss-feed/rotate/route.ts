import type { NextRequest } from "next/server";

import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

interface RequestFeedLink {
  readonly url: string;
}

export const POST = createPostHandler<RequestFeedLink>(
  async (_request: NextRequest, _body: unknown, token: string) =>
    apiPost<RequestFeedLink>(
      "/api/students/change-requests/rss-feed/rotate",
      token,
    ),
);
