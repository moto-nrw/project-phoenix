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

export const PUT = createPutHandler<unknown, { opt_out?: boolean }>(
  async (
    _request: NextRequest,
    body: { opt_out?: boolean },
    token: string,
    _params: Record<string, unknown>,
  ) => {
    return await updateBirthdayOptOut(token, body.opt_out === true);
  },
);
