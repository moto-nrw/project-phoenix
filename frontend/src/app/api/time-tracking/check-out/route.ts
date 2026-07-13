import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

interface CheckOutRequest {
  reason?: string;
}

/**
 * POST /api/time-tracking/check-out
 * Check out from work. The body is optional; `reason` carries the F9
 * deviation reason when the backend demanded one. Without a reason the
 * request is forwarded body-less, exactly like before the F9 extension.
 */
export const POST = createPostHandler<unknown, CheckOutRequest>(
  async (_request: NextRequest, body: CheckOutRequest, token: string) => {
    const response = body.reason
      ? await apiPost<{ data: unknown }>(
          "/api/time-tracking/check-out",
          token,
          { reason: body.reason },
        )
      : await apiPost<{ data: unknown }>("/api/time-tracking/check-out", token);
    return response.data;
  },
);
