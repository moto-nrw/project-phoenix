import type { NextRequest } from "next/server";
import {
  fetchBirthdayOptOut,
  updateBirthdayOptOut,
} from "~/lib/birthdays-api.server";
import { createGetHandler, createPutHandler } from "~/lib/route-wrapper.server";

/**
 * GET/PUT /api/birthdays/opt-out — the caller's own birthday display setting
 * (#1542). The backend resolves the staff row from the token, so the body
 * carries the preference only, never a person.
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    _params: Record<string, unknown>,
  ) => {
    return await fetchBirthdayOptOut(token);
  },
);

export const PUT = createPutHandler<unknown, { opt_out?: unknown }>(
  async (
    _request: NextRequest,
    body: { opt_out?: unknown },
    token: string,
    _params: Record<string, unknown>,
  ) => {
    // Reject anything that is not a boolean instead of coercing it. A missing
    // or malformed field used to read as `false`, which silently CLEARED an
    // existing opt-out and put the person's name back on the shared dashboard
    // — the one outcome this setting exists to prevent.
    if (typeof body.opt_out !== "boolean") {
      throw new Error("API error (400): opt_out must be a boolean");
    }
    return await updateBirthdayOptOut(token, body.opt_out);
  },
);
