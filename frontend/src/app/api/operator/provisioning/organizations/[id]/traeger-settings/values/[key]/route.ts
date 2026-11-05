import { bffError } from "~/lib/api-response-error.server";
import { proxyDelete, proxyPut } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Träger-Wert einer Einstellung aus dem Operator-Dashboard setzen oder
// entfernen (#5748). Same key format as the Träger portal route.
const VALID_KEY_PATTERN = /^[a-z0-9_.]{1,255}$/;

function settingValuePath(params: Record<string, unknown>): string {
  const key = params.key;
  if (typeof key !== "string" || !VALID_KEY_PATTERN.test(key)) {
    throw bffError(400, "Invalid settings key format");
  }
  return `/operator/organizations/${requirePathSegmentParam(params)}/traeger-settings/values/${key}`;
}

export const PUT = proxyPut(settingValuePath);

export const DELETE = proxyDelete(settingValuePath);
