import type { NextRequest } from "next/server";

import {
  createParentPutHandler,
  parentApiPut,
} from "~/lib/parent/route-wrapper.server";
import { bffError } from "~/lib/api-response-error.server";

/** Bounds the path segment before it reaches the backend. */
const VALID_TYPE_PATTERN = /^[a-z0-9_]{1,64}$/;

/**
 * PUT /api/parent/me/notification-preferences/{type} — record one decision,
 * applied to every school the family belongs to.
 */
export const PUT = createParentPutHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const type = params.type as string;
    if (!VALID_TYPE_PATTERN.test(type)) {
      throw bffError(400, "Invalid notification type format");
    }
    return parentApiPut(
      `/parent/me/notification-preferences/${type}`,
      token,
      body,
    );
  },
);
