import type { NextRequest } from "next/server";

import { apiGet, apiPost } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";

interface RequestFeedStatus {
  readonly active: boolean;
}

interface RequestFeedLink {
  readonly url: string;
}

const backendPath = "/api/students/change-requests/rss-feed";

export const GET = createGetHandler<RequestFeedStatus>(
  async (_request: NextRequest, token: string) =>
    apiGet<RequestFeedStatus>(backendPath, token),
);

export const POST = createPostHandler<RequestFeedLink>(
  async (_request: NextRequest, _body: unknown, token: string) =>
    apiPost<RequestFeedLink>(backendPath, token),
);
