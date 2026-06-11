import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string) => {
    return apiPost<unknown>("/api/timetable/lists/options", token, body ?? {});
  },
);
