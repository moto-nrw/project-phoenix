import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/messages/threads/{threadId} → backend. Returns the full
 * thread (subject, child/guardian metadata, messages oldest-first).
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const threadId = params.threadId as string;
    if (!threadId) {
      throw new Error("Thread ID is required");
    }
    const response = await apiGet<{ data: unknown }>(
      `/api/messages/threads/${threadId}`,
      token,
    );
    return response.data;
  },
);

/**
 * Proxy POST /api/messages/threads/{threadId} → backend. Sends a staff reply
 * ({"body": "..."}) and returns the full updated message list.
 */
export const POST = createPostHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const threadId = params.threadId as string;
    if (!threadId) {
      throw new Error("Thread ID is required");
    }
    const response = await apiPost<{ data: unknown }>(
      `/api/messages/threads/${threadId}`,
      token,
      body,
    );
    return response.data;
  },
);
