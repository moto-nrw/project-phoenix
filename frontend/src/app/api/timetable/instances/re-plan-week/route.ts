import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/timetable/instances/re-plan-week",
      token,
      body ?? {},
    );
    return response.data;
  },
);
