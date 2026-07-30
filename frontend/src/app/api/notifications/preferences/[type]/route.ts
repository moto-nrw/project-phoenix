import type { NextRequest } from "next/server";

import { apiPut } from "~/lib/api-helpers.server";
import { createPutHandler } from "~/lib/route-wrapper.server";

/** Bounds the path segment before it reaches the backend. */
const VALID_TYPE_PATTERN = /^[a-z0-9_]{1,64}$/;

/**
 * PUT /api/notifications/preferences/{type} — record one decision.
 */
export const PUT = createPutHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const type = params.type as string;
    if (!VALID_TYPE_PATTERN.test(type)) {
      throw new Error("API error (400): Invalid notification type format");
    }
    return apiPut(`/api/notifications/preferences/${type}`, token, body);
  },
);
