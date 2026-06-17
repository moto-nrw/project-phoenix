import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy POST /api/messages/threads → backend. Staff starts a new parent-OGS
 * thread ({"student_id","guardian_account_id","subject","body"}). The backend
 * creates the thread plus its first message and returns the full ThreadDetail.
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
