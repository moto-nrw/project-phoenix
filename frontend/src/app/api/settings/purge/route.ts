// Settings purge API route
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

interface PurgeBody {
  days: number;
}

export const POST = createPostHandler<unknown, PurgeBody>(
  async (_request: NextRequest, body: PurgeBody, token: string) => {
    return await apiPost("/api/settings/purge", token, body);
  },
);
