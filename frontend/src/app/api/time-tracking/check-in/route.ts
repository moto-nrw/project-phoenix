import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

interface CheckInRequest {
  status: "present" | "home_office";
}

/**
 * POST /api/time-tracking/check-in
 * Check in for work with specified status
 */
export const POST = createPostHandler<unknown, CheckInRequest>(
  async (_request: NextRequest, body: CheckInRequest, token: string) => {
    const response = await apiPost("/api/time-tracking/check-in", token, body);
    return response;
  },
);
