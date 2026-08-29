import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/staff-messages/threads/{threadID} → backend. Returns the
 * conversation (counterpart + messages oldest-first) and marks it read.
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const threadID = params.threadID as string;
    if (!threadID) {
      throw new Error("Thread ID is required");
    }
    const response = await apiGet<{ data: unknown }>(
      `/api/staff-messages/threads/${threadID}`,
      token,
    );
    return response.data;
  },
);

/**
 * Proxy POST /api/staff-messages/threads/{threadID} → backend. Sends one
 * message ({"body": "..."}) and returns it.
 */
export const POST = createPostHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const threadID = params.threadID as string;
    if (!threadID) {
      throw new Error("Thread ID is required");
    }
    const response = await apiPost<{ data: unknown }>(
      `/api/staff-messages/threads/${threadID}`,
      token,
      body,
    );
    return response.data;
  },
);
