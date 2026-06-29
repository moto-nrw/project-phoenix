import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy POST /api/messages/threads → backend. Staff sends the first message to
 * a guardian about a child ({"student_id","guardian_account_id","body"}). The
 * backend get-or-creates the conversation and returns the full ThreadDetail.
 */
export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/messages/threads",
      token,
      body,
    );
    return response.data;
  },
);
