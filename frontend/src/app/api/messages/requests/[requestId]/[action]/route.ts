import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

// The staff actions on a parent change-request. One handler with an allowlist,
// instead of byte-identical route files (confirm / reject) that differed only in
// the trailing path segment.
const ALLOWED_ACTIONS = new Set(["confirm", "reject"]);

export const POST = createPostHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const requestId = params.requestId as string;
    if (!requestId) throw new Error("Request ID is required");
    const action = params.action as string;
    if (!ALLOWED_ACTIONS.has(action)) {
      throw new Error("Unknown request action");
    }
    const response = await apiPost<{ data: unknown }>(
      `/api/messages/requests/${requestId}/${action}`,
      token,
      body,
    );
    return response.data;
  },
);
