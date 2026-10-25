import { bffError } from "~/lib/api-response-error.server";
import { proxyDelete, proxyPut } from "~/lib/traeger/route-wrapper.server";

// Träger-Wert einer Einstellung setzen oder entfernen (#4652, ADR 0086).
// Same key format as the school settings routes (src/app/api/settings/).
const VALID_KEY_PATTERN = /^[a-z0-9_.]{1,255}$/;

function settingValuePath(params: Record<string, unknown>): string {
  const key = params.key;
  if (typeof key !== "string" || !VALID_KEY_PATTERN.test(key)) {
    throw bffError(400, "Invalid settings key format");
  }
  return `/traeger/settings/values/${key}`;
}

export const PUT = proxyPut(settingValuePath);

export const DELETE = proxyDelete(settingValuePath);
