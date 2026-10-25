import type { NextRequest } from "next/server";
import { bffError } from "~/lib/api-response-error.server";
import {
  createTraegerDeleteHandler,
  createTraegerPutHandler,
  traegerApiDelete,
  traegerApiPut,
} from "~/lib/traeger/route-wrapper.server";

// Träger-Wert einer Einstellung setzen oder entfernen (#4652, ADR 0086).
// Same key format as the school settings routes (src/app/api/settings/).
const VALID_KEY_PATTERN = /^[a-z0-9_.]{1,255}$/;

function settingKey(params: Record<string, unknown>): string {
  const key = params.key;
  if (typeof key !== "string" || !VALID_KEY_PATTERN.test(key)) {
    throw bffError(400, "Invalid settings key format");
  }
  return key;
}

export const PUT = createTraegerPutHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) =>
    traegerApiPut(
      `/traeger/settings/values/${settingKey(params)}`,
      token,
      body,
    ),
);

export const DELETE = createTraegerDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) =>
    traegerApiDelete(`/traeger/settings/values/${settingKey(params)}`, token),
);
