import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

/**
 * Handler for POST /api/active/schulhof/supervise
 * Toggles Schulhof supervision for the current user
 * Request body: { action: "start" | "stop" }
 */
export const POST = createPostHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
  ) => {
    // Validate and pass through the action
    const { action } = body as { action?: string };

    if (action !== "start" && action !== "stop") {
      throw new Error("Action must be 'start' or 'stop'");
    }

    return await apiPost("/api/active/schulhof/supervise", token, { action });
  },
);
