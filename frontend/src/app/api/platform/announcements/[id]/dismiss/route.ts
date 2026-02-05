import type { NextRequest } from "next/server";
import { createPostHandler } from "~/lib/route-wrapper";
import { apiPost } from "~/lib/api-helpers";

export const POST = createPostHandler(
  async (_request: NextRequest, _body: unknown, token: string, params) => {
    const id = params.id as string;
    return await apiPost(`/api/platform/announcements/${id}/dismiss`, token);
  },
);
