import type { NextRequest } from "next/server";
import { createPostHandler } from "~/lib/route-wrapper.server";
import { apiPost } from "~/lib/api-helpers.server";

export const POST = createPostHandler(
  async (_request: NextRequest, _body: unknown, token: string, params) => {
    const id = params.id as string;
    return await apiPost(`/api/platform/announcements/${id}/seen`, token);
  },
);
